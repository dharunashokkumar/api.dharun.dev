#!/usr/bin/env bash
# Builds the sandbox images: one ext4 file per image, used by sandboxd as a microVM's root
# file system. Run as root on the sandbox VM (Debian 13) by install.sh; it downloads
# Debian packages and the distributions' root file systems, so it takes a while.
#
#   build-images.sh <out dir> <bin dir> [image ...]
#
# <bin dir> holds sandbox-agent and sandbox-browser. With no image named, all are built:
# code browser debian alpine arch fedora, and the scratch disk.
set -euo pipefail

out="${1:?usage: build-images.sh <out dir> <bin dir> [image ...]}"
bin="${2:?usage: build-images.sh <out dir> <bin dir> [image ...]}"
shift 2
images=("$@")
[ ${#images[@]} -gt 0 ] || images=(scratch code browser debian alpine arch fedora)

mirror="${DEBIAN_MIRROR:-http://deb.debian.org/debian}"
suite=trixie
work="$(mktemp -d /var/tmp/sandbox-images.XXXXXX)"
trap 'umount -R "$work"/*/proc 2>/dev/null || true; rm -rf "$work"' EXIT
mkdir -p "$out"

# Distribution root file systems, checked against the publishers' own checksums.
alpine_url=https://dl-cdn.alpinelinux.org/alpine/v3.24/releases/x86_64/alpine-minirootfs-3.24.2-x86_64.tar.gz
alpine_sha=c5ca053cfe1d85c5b96dff8b9bc57045f7f184a30ffb6b65776409ca90388677
arch_url=https://geo.mirror.pkgbuild.com/iso/2026.10.01/archlinux-bootstrap-2026.10.01-x86_64.tar.zst
arch_sha=0c9c97b7a58a8a023c9e07f84cb307a9e8006e65d8fc4c17b83ddc8010b45c10
fedora_url=https://dl.fedoraproject.org/pub/fedora/linux/releases/44/Container/x86_64/images/Fedora-Container-Base-Generic-44-1.7.x86_64.oci.tar.xz
fedora_sha=75200f5752a74a21a616ca9a75e25beb594e2e117a0195c54f87c0b3e3974d1b

log() { echo "build-images: $*"; }

fetch() { # url sha256 file
  curl -fsSL --retry 3 -o "$3" "$1"
  echo "$2  $3" | sha256sum -c --quiet - || { echo "ERROR: checksum of $1 does not match" >&2; exit 1; }
}

debian_root() { # dir packages...
  local dir="$1"
  shift
  local packages
  packages=$(IFS=,; echo "$*")
  mmdebstrap --quiet --variant=minbase --include="$packages" \
    --dpkgopt='path-exclude=/usr/share/doc/*' --dpkgopt='path-exclude=/usr/share/man/*' \
    --dpkgopt='path-exclude=/usr/share/info/*' --dpkgopt='path-include=/usr/share/doc/*/copyright' \
    "$suite" "$dir" "$mirror"
  rm -rf "$dir"/var/cache/apt/* "$dir"/var/lib/apt/lists/*
}

# finish puts the agent in, adds the sandbox user and the places the agent mounts on.
finish() { # dir shell
  local dir="$1" shell="$2"
  # At the root, a real directory in every distribution (/sbin is a link in some).
  install -m 0755 "$bin/sandbox-agent" "$dir/sandbox-agent"
  mkdir -p "$dir"/{work,scratch,tmp,proc,sys,dev,run,home/sandbox}
  chmod 1777 "$dir/tmp"
  if ! grep -q '^[^:]*:[^:]*:1000:' "$dir/etc/passwd"; then
    echo "sandbox:x:1000:1000:Sandbox:/home/sandbox:$shell" >> "$dir/etc/passwd"
    grep -q '^sandbox:' "$dir/etc/group" || echo "sandbox:x:1000:" >> "$dir/etc/group"
    if [ -f "$dir/etc/shadow" ]; then echo "sandbox:!:20000::::::" >> "$dir/etc/shadow"; fi
  fi
  chown 1000:1000 "$dir/home/sandbox" "$dir/work"
  printf '127.0.0.1 localhost sandbox\n::1 localhost\n' > "$dir/etc/hosts"
  echo sandbox > "$dir/etc/hostname"
  rm -f "$dir/etc/resolv.conf" && : > "$dir/etc/resolv.conf"
  cat > "$dir/etc/motd" <<'MOTD'

  India API sandbox: a throwaway Linux machine. Everything here is deleted when the
  session ends. No internet; in a code session with network on, each language's package
  registry works (pip, npm, go, gem, composer). Be kind: abuse ends the account.

MOTD
}

make_ext4() { # dir file extra_mb
  local dir="$1" file="$2" extra="$3" size
  size=$(( $(du -sm --apparent-size "$dir" | cut -f1) * 115 / 100 + extra ))
  rm -f "$file.new"
  truncate -s "${size}M" "$file.new"
  # A shared read-only image needs no journal; a writable one keeps it.
  local journal=(-O ^has_journal)
  [ "$extra" -le 64 ] || journal=()
  mkfs.ext4 -q -F -L root -m 0 "${journal[@]}" -d "$dir" "$file.new"
  mv "$file.new" "$file"
  log "$(basename "$file"): ${size} MB"
}

build_scratch() {
  # Each code or browser VM gets a copy of this as its writable disk.
  rm -f "$out/scratch.ext4.new"
  truncate -s 1G "$out/scratch.ext4.new"
  mkfs.ext4 -q -F -L scratch -m 0 "$out/scratch.ext4.new"
  mv "$out/scratch.ext4.new" "$out/scratch.ext4"
  log "scratch.ext4: 1 GB, sparse"
}

build_code() {
  local dir="$work/code"
  debian_root "$dir" bash ca-certificates procps less nano vim-tiny file unzip xz-utils curl git make \
    python3 python3-pip python3-venv python3-numpy python3-pandas python3-matplotlib python3-requests python3-yaml \
    nodejs npm golang-go gcc g++ libc6-dev ruby php-cli php-mbstring php-xml composer
  finish "$dir" /bin/bash
  # The home every code VM starts from, under its own changes: a Go build cache for the
  # standard library, so the first go run does not compile it.
  mkdir -p "$dir/opt/sandbox-home"
  chown 1000:1000 "$dir/opt/sandbox-home"
  mount -t proc proc "$dir/proc"
  chroot "$dir" /usr/sbin/runuser -u sandbox -- env HOME=/opt/sandbox-home GOCACHE=/opt/sandbox-home/.cache/go-build \
    GOTOOLCHAIN=local GOFLAGS=-mod=mod GOPROXY=off /usr/bin/go build std
  chroot "$dir" /usr/sbin/runuser -u sandbox -- env HOME=/opt/sandbox-home MPLBACKEND=Agg \
    python3 -c 'import matplotlib.pyplot, pandas, numpy' 2>/dev/null || true
  umount "$dir/proc"
  make_ext4 "$dir" "$out/code.ext4" 64
}

build_browser() {
  local dir="$work/browser"
  debian_root "$dir" ca-certificates chromium fonts-noto-core fonts-noto-ui-core fonts-noto-cjk fonts-noto-color-emoji fonts-liberation2
  finish "$dir" /bin/sh
  install -m 0755 "$bin/sandbox-browser" "$dir/usr/local/bin/sandbox-browser"
  make_ext4 "$dir" "$out/browser.ext4" 64
}

build_debian() {
  local dir="$work/debian"
  debian_root "$dir" bash procps less nano vim-tiny file tree htop ncurses-term python3-minimal man-db
  finish "$dir" /bin/bash
  make_ext4 "$dir" "$out/debian.ext4" 512
}

build_alpine() {
  local dir="$work/alpine"
  mkdir -p "$dir"
  fetch "$alpine_url" "$alpine_sha" "$work/alpine.tar.gz"
  tar -xzf "$work/alpine.tar.gz" -C "$dir"
  finish "$dir" /bin/sh
  make_ext4 "$dir" "$out/alpine.ext4" 512
}

build_arch() {
  local dir="$work/arch"
  mkdir -p "$work/arch-unpack"
  fetch "$arch_url" "$arch_sha" "$work/arch.tar.zst"
  tar --zstd -xf "$work/arch.tar.zst" -C "$work/arch-unpack"
  mv "$work/arch-unpack/root.x86_64" "$dir"
  finish "$dir" /bin/bash
  make_ext4 "$dir" "$out/arch.ext4" 512
}

build_fedora() {
  local dir="$work/fedora" oci="$work/fedora-oci"
  mkdir -p "$dir" "$oci"
  fetch "$fedora_url" "$fedora_sha" "$work/fedora.tar.xz"
  tar -xJf "$work/fedora.tar.xz" -C "$oci"
  local manifest layer
  manifest=$(jq -r '.manifests[0].digest' "$oci/index.json" | cut -d: -f2)
  for layer in $(jq -r '.layers[].digest' "$oci/blobs/sha256/$manifest" | cut -d: -f2); do
    tar -xf "$oci/blobs/sha256/$layer" -C "$dir"
  done
  finish "$dir" /bin/bash
  make_ext4 "$dir" "$out/fedora.ext4" 512
}

for image in "${images[@]}"; do
  log "building $image"
  case "$image" in
    scratch) build_scratch ;;
    code) build_code ;;
    browser) build_browser ;;
    debian) build_debian ;;
    alpine) build_alpine ;;
    arch) build_arch ;;
    fedora) build_fedora ;;
    *) echo "ERROR: unknown image $image" >&2; exit 1 ;;
  esac
done
log "done"
