#!/usr/bin/env bash
# Installs the sandbox controller on its own VM: Debian 13, nested virtualization (/dev/kvm),
# on an isolated network that only the API container can reach. Safe to run again: it
# keeps the secret, the settings and the images, and builds only images that are missing.
#
#   API_ADDR=<the API container's address> bash install.sh [sandbox bundle .tar.gz]
#
# Without a bundle, the newest release's is downloaded. SKIP_IMAGES=1 leaves the images
# out (they take a while: about 2 GB of packages to download).
set -euo pipefail

API_ADDR="${API_ADDR:?set API_ADDR to the address of the API container, the only one that may reach Postgres}"
LISTEN_ADDR="${LISTEN_ADDR:-$(hostname -I | awk '{print $1}')}"
REPO="${SANDBOX_REPO:-dharunashokkumar/api.dharun.dev}"
HOME_DIR=/opt/sandbox

# Pinned: Firecracker's release and its CI guest kernel (Linux 6.1, every driver built in).
FIRECRACKER_VERSION=v1.17.0
FIRECRACKER_SHA=06094a1108ae9e82aa4c23a775aa92758f53f1175d422270d9d6162cb9ade558
KERNEL_URL="${KERNEL_URL:-https://s3.amazonaws.com/spec.ccfc.min/firecracker-ci/20261016-5732b8331e1e-0/x86_64/vmlinux-6.1.188}"
KERNEL_SHA=c16288854c798ded46c45b3ccab4bc85b8041552ab50f899fecbad550a641e92

log() { echo "sandbox-install: $*"; }
[ "$(id -u)" = 0 ] || { echo "run as root" >&2; exit 1; }
[ -c /dev/kvm ] || { echo "ERROR: /dev/kvm is missing: the VM needs CPU type host and nested virtualization" >&2; exit 1; }

log "packages"
export DEBIAN_FRONTEND=noninteractive
apt-get update -q
apt-get install -y -q --no-install-recommends mmdebstrap e2fsprogs curl ca-certificates jq zstd xz-utils \
  postgresql util-linux procps

fetch() { # url sha256 file
  curl -fsSL --retry 3 -o "$3.part" "$1"
  echo "$2  $3.part" | sha256sum -c --quiet - || { echo "ERROR: checksum of $1 does not match" >&2; exit 1; }
  mv "$3.part" "$3"
}

mkdir -p "$HOME_DIR/bin" "$HOME_DIR/images" "$HOME_DIR/jail" "$HOME_DIR/releases" /var/lib/sandboxd /etc/sandboxd
chmod 700 /etc/sandboxd /var/lib/sandboxd "$HOME_DIR/jail"

if ! "$HOME_DIR/bin/firecracker" --version 2>/dev/null | grep -q "${FIRECRACKER_VERSION#v}"; then
  log "Firecracker $FIRECRACKER_VERSION"
  work="$(mktemp -d)"
  fetch "https://github.com/firecracker-microvm/firecracker/releases/download/$FIRECRACKER_VERSION/firecracker-$FIRECRACKER_VERSION-x86_64.tgz" \
    "$FIRECRACKER_SHA" "$work/firecracker.tgz"
  tar -xzf "$work/firecracker.tgz" -C "$work"
  install -m 0755 "$work/release-$FIRECRACKER_VERSION-x86_64/firecracker-$FIRECRACKER_VERSION-x86_64" "$HOME_DIR/bin/firecracker"
  install -m 0755 "$work/release-$FIRECRACKER_VERSION-x86_64/jailer-$FIRECRACKER_VERSION-x86_64" "$HOME_DIR/bin/jailer"
  rm -rf "$work"
fi
if ! echo "$KERNEL_SHA  $HOME_DIR/vmlinux" | sha256sum -c --quiet - 2>/dev/null; then
  log "guest kernel"
  fetch "$KERNEL_URL" "$KERNEL_SHA" "$HOME_DIR/vmlinux"
fi

log "sandbox release"
bundle="${1:-}"
if [ -z "$bundle" ]; then
  bundle="$(mktemp -d)/sandbox-linux-amd64.tar.gz"
  base="https://github.com/$REPO/releases/latest/download"
  curl -fsSL --retry 3 -o "$bundle" "$base/sandbox-linux-amd64.tar.gz"
  sum="$(curl -fsSL --retry 3 "$base/sandbox-linux-amd64.tar.gz.sha256" | cut -d' ' -f1)"
  echo "$sum  $bundle" | sha256sum -c --quiet -
fi
stage="$(mktemp -d)"
tar -xzf "$bundle" -C "$stage"
version="$(cat "$stage/sandbox/VERSION")"
rm -rf "$HOME_DIR/releases/$version"
mv "$stage/sandbox" "$HOME_DIR/releases/$version"
ln -sfn "releases/$version" "$HOME_DIR/current"
for program in sandboxd sandbox-agent sandbox-browser; do
  install -m 0755 "$HOME_DIR/current/$program" "$HOME_DIR/bin/$program"
done

if [ ! -s /etc/sandboxd/secret ]; then
  log "a new secret for the API's link"
  (umask 077 && head -c 32 /dev/urandom | base64 | tr -d '/+=' > /etc/sandboxd/secret)
fi
if [ ! -f /etc/sandboxd/env ]; then
  sed -e "s/^LISTEN=.*/LISTEN=$LISTEN_ADDR:7070/" -e "s/^METRICS_LISTEN=.*/METRICS_LISTEN=$LISTEN_ADDR:9465/" \
    "$HOME_DIR/current/sandboxd.env.example" > /etc/sandboxd/env
  chmod 600 /etc/sandboxd/env
fi

log "postgres"
for pg_dir in /etc/postgresql/*/main; do :; done
cat > "$pg_dir/conf.d/sandbox.conf" <<EOF
# India API throwaway databases (deploy/sandbox/install.sh). Only the API container
# reaches this server, over the private link; callers come through its WebSocket.
listen_addresses = 'localhost,$LISTEN_ADDR'
max_connections = 200
password_encryption = scram-sha-256
ssl = off
shared_buffers = 128MB
statement_timeout = 30s
idle_session_timeout = 15min
log_connections = off
EOF
hba_line="host sameuser all $API_ADDR/32 scram-sha-256"
grep -qF "$hba_line" "$pg_dir/pg_hba.conf" || echo "$hba_line" >> "$pg_dir/pg_hba.conf"
systemctl restart postgresql

if [ "${SKIP_IMAGES:-}" != 1 ]; then
  missing=()
  for image in scratch code browser debian alpine arch fedora; do
    [ -f "$HOME_DIR/images/$image.ext4" ] || missing+=("$image")
  done
  if [ ${#missing[@]} -gt 0 ]; then
    log "building images: ${missing[*]} (this takes a while)"
    bash "$HOME_DIR/current/build-images.sh" "$HOME_DIR/images" "$HOME_DIR/bin" "${missing[@]}"
  fi
  sha256sum "$HOME_DIR/current/build-images.sh" | cut -d' ' -f1 > /var/lib/sandboxd/images.built-with
  sha256sum "$HOME_DIR/bin/sandbox-agent" "$HOME_DIR/bin/sandbox-browser" > /var/lib/sandboxd/images.patched-with
fi

log "services"
install -m 0644 "$HOME_DIR/current/sandboxd.service" "$HOME_DIR/current/sandbox-update.service" \
  "$HOME_DIR/current/sandbox-update.timer" /etc/systemd/system/
systemctl daemon-reload
systemctl enable --now sandboxd.service sandbox-update.timer
systemctl restart sandboxd.service

for _ in $(seq 1 30); do
  if curl -fsS -m 3 -H "Authorization: Bearer $(cat /etc/sandboxd/secret)" "http://$LISTEN_ADDR:7070/status" >/dev/null 2>&1; then
    log "sandboxd $version is up on $LISTEN_ADDR:7070"
    log "on the API container, put in its env file:"
    log "  SANDBOX_URL=http://$LISTEN_ADDR:7070"
    log "  SANDBOX_PG_ADDR=$LISTEN_ADDR:5432"
    log "  SANDBOX_SECRET=<the content of /etc/sandboxd/secret>"
    exit 0
  fi
  sleep 1
done
echo "ERROR: sandboxd did not answer; see journalctl -u sandboxd" >&2
exit 1
