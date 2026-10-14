#!/usr/bin/env bash
# Pull-based deploy. Installs the newest GitHub release when it differs from the running
# one, restarts the service, and rolls back if the new version fails its health check.
# Nothing connects in: the box only makes outbound HTTPS requests to GitHub.
set -euo pipefail

REPO="${API_REPO:-dharunashokkumar/api.dharun.dev}"
RELEASE_URL="${API_RELEASE_URL:-https://github.com/$REPO/releases/latest/download}"
HOME_DIR="${API_HOME:-/opt/dharun-api}"
HEALTH_URL="${API_HEALTH_URL:-http://127.0.0.1:${PORT:-8080}/healthz}"
RESTART_CMD="${API_RESTART_CMD:-systemctl restart dharun-api.service}"
HEALTH_TRIES="${API_HEALTH_TRIES:-20}"
BUNDLE=dharun-api-linux-amd64.tar.gz
KEEP_RELEASES=3

log() { printf 'dharun-api-update: %s\n' "$*"; }

# The service answers and reports exactly this version.
healthy() {
  for _ in $(seq 1 "$HEALTH_TRIES"); do
    if curl -fsS -m 3 "$HEALTH_URL" 2>/dev/null | grep -q "\"version\":\"$1\""; then return 0; fi
    sleep 1
  done
  return 1
}

# Atomic switch of the `current` symlink to releases/<name>.
activate() {
  ln -sfn "releases/$1" "$HOME_DIR/current.new"
  mv -Tf "$HOME_DIR/current.new" "$HOME_DIR/current"
}

latest="$(curl -fsSL -m 30 "$RELEASE_URL/VERSION" | tr -d '[:space:]')"
if ! [[ "$latest" =~ ^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.]+)?$ ]]; then
  log "ERROR: release VERSION is not a version number: '${latest:0:40}'"
  exit 1
fi

running="$(cat "$HOME_DIR/current/VERSION" 2>/dev/null || true)"
[ "$latest" = "$running" ] && exit 0

if [ -e "$HOME_DIR/failed/$latest" ]; then
  log "skipping $latest: it failed its health check earlier (remove $HOME_DIR/failed/$latest to retry)"
  exit 0
fi

mkdir -p "$HOME_DIR/releases" "$HOME_DIR/failed"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

log "installing $latest (running: ${running:-none})"
curl -fsSL -m 600 -o "$work/$BUNDLE" "$RELEASE_URL/$BUNDLE"
curl -fsSL -m 30 -o "$work/$BUNDLE.sha256" "$RELEASE_URL/$BUNDLE.sha256"
(cd "$work" && sha256sum -c --quiet "$BUNDLE.sha256")

mkdir "$work/release"
tar -xzf "$work/$BUNDLE" -C "$work/release" --strip-components=1 --no-same-owner
bundled="$(cat "$work/release/VERSION")"
if [ "$bundled" != "$latest" ]; then
  log "ERROR: bundle is $bundled, expected $latest (a release was published mid-download; the next run retries)"
  exit 1
fi

rm -rf "${HOME_DIR:?}/releases/$latest"
mv "$work/release" "$HOME_DIR/releases/$latest"
chmod -R a+rX "$HOME_DIR/releases/$latest"

activate "$latest"

# A restart that fails counts like a failed health check: under `set -e` it would end the
# script here, leaving the new release active and looking deployed to the next run.
if $RESTART_CMD && healthy "$latest"; then
  log "deployed $latest"
  # Keep the newest few releases; the one just replaced is among them, ready for a manual rollback.
  find "$HOME_DIR/releases" -mindepth 1 -maxdepth 1 -type d -printf '%T@ %f\n' | sort -rn |
    tail -n +"$((KEEP_RELEASES + 1))" | while read -r _ old; do
      [ "$old" = "$latest" ] || rm -rf "${HOME_DIR:?}/releases/$old"
    done
  exit 0
fi

log "ERROR: $latest did not restart or did not pass its health check"
touch "$HOME_DIR/failed/$latest"
if [ -n "$running" ] && [ -d "$HOME_DIR/releases/$running" ]; then
  activate "$running"
  if $RESTART_CMD && healthy "$running"; then
    log "rolled back to $running"
  else
    log "ERROR: rollback to $running is not healthy either"
  fi
fi
exit 1
