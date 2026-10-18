#!/usr/bin/env bash
# Pull-based update of the sandbox VM, like the API's: installs the newest release's
# sandbox bundle when it differs from the running one, patches a new agent or browser
# helper into the images, rebuilds the images when their recipe changed, restarts the
# controller and rolls back if it does not come up. Nothing connects in.
set -euo pipefail

REPO="${SANDBOX_REPO:-dharunashokkumar/api.dharun.dev}"
BASE="${SANDBOX_RELEASE_URL:-https://github.com/$REPO/releases/latest/download}"
HOME_DIR="${SANDBOX_HOME:-/opt/sandbox}"
STATE=/var/lib/sandboxd
LISTEN="${LISTEN:-127.0.0.1:7070}"
KEEP=3

log() { echo "sandbox-update: $*"; }

healthy() { # version
  for _ in $(seq 1 30); do
    if curl -fsS -m 3 -H "Authorization: Bearer $(cat /etc/sandboxd/secret)" "http://$LISTEN/status" 2>/dev/null |
      grep -q "\"version\":\"$1\""; then return 0; fi
    sleep 1
  done
  return 1
}

install_binaries() {
  for program in sandboxd sandbox-agent sandbox-browser; do
    install -m 0755 "$HOME_DIR/current/$program" "$HOME_DIR/bin/$program.new"
    mv "$HOME_DIR/bin/$program.new" "$HOME_DIR/bin/$program"
  done
}

latest="$(curl -fsSL -m 30 "$BASE/VERSION" | tr -d '[:space:]')"
[[ "$latest" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || { log "ERROR: release VERSION is not a version: '${latest:0:40}'"; exit 1; }
current="$(cat "$HOME_DIR/current/VERSION" 2>/dev/null || true)"
[ "$latest" != "$current" ] || exit 0
if [ -f "$STATE/failed-$latest" ]; then exit 0; fi

log "installing $latest (running ${current:-nothing})"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
curl -fsSL -m 300 -o "$work/sandbox.tar.gz" "$BASE/sandbox-linux-amd64.tar.gz"
sum="$(curl -fsSL -m 30 "$BASE/sandbox-linux-amd64.tar.gz.sha256" | cut -d' ' -f1)"
echo "$sum  $work/sandbox.tar.gz" | sha256sum -c --quiet - || { log "ERROR: checksum mismatch for $latest"; exit 1; }
tar -xzf "$work/sandbox.tar.gz" -C "$work"
[ "$(cat "$work/sandbox/VERSION")" = "$latest" ] || { log "ERROR: the bundle says it is $(cat "$work/sandbox/VERSION")"; exit 1; }
rm -rf "$HOME_DIR/releases/$latest"
mv "$work/sandbox" "$HOME_DIR/releases/$latest"

previous="$(readlink "$HOME_DIR/current" || true)"
ln -sfn "releases/$latest" "$HOME_DIR/current.new" && mv -Tf "$HOME_DIR/current.new" "$HOME_DIR/current"
install_binaries

# A new agent or browser helper goes into the images; a new recipe rebuilds them.
if ! sha256sum -c --quiet "$STATE/images.patched-with" >/dev/null 2>&1; then
  log "patching the images"
  bash "$HOME_DIR/current/patch-images.sh" "$HOME_DIR/images" "$HOME_DIR/bin"
  sha256sum "$HOME_DIR/bin/sandbox-agent" "$HOME_DIR/bin/sandbox-browser" > "$STATE/images.patched-with"
fi
recipe="$(sha256sum "$HOME_DIR/current/build-images.sh" | cut -d' ' -f1)"
if [ "$recipe" != "$(cat "$STATE/images.built-with" 2>/dev/null)" ]; then
  log "the image recipe changed: rebuilding the images in the background"
  systemd-run --unit="sandbox-images-$latest" --nice=19 --collect sh -c \
    "bash '$HOME_DIR/current/build-images.sh' '$HOME_DIR/images' '$HOME_DIR/bin' && echo $recipe > $STATE/images.built-with" || true
fi

systemctl restart sandboxd.service
if healthy "$latest"; then
  log "running $latest"
  find "$HOME_DIR/releases" -mindepth 1 -maxdepth 1 -printf '%T@ %p\n' | sort -rn | tail -n +$((KEEP + 1)) | cut -d' ' -f2- | xargs -r rm -rf
  exit 0
fi

log "ERROR: $latest did not come up; rolling back to ${previous:-nothing}"
touch "$STATE/failed-$latest"
if [ -n "$previous" ]; then
  ln -sfn "$previous" "$HOME_DIR/current.new" && mv -Tf "$HOME_DIR/current.new" "$HOME_DIR/current"
  install_binaries
  systemctl restart sandboxd.service
fi
exit 1
