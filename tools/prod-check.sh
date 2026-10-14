#!/usr/bin/env bash
# Run by CI after a release is published. A server installs a release on its own (its
# updater looks every two minutes and rolls back when the health check fails), so CI only
# knows it is live once the public site reports it. This waits for that, then checks the
# routes a deploy must not break against the public site, and that the datasets only a
# server fetches are there. It only reads the public site and needs no credentials.
#   bash tools/prod-check.sh https://api.dharun.dev 1.1.42
set -euo pipefail

base="${1:?usage: prod-check.sh <origin> <version>}"
want="${2:?usage: prod-check.sh <origin> <version>}"
wait_minutes="${PROD_WAIT_MINUTES:-25}"

# The build number of a 1.1.<run> release, or nothing for any other version.
build() { [[ "$1" =~ ^1\.1\.([0-9]+)$ ]] && echo "${BASH_REMATCH[1]}"; }

deadline=$((SECONDS + wait_minutes * 60))
while :; do
  health="$(curl -fsS -m 15 "$base/healthz" 2>/dev/null || true)"
  running=""
  if [[ "$health" =~ ^\{\"status\":\"ok\",\"version\":\"([^\"]+)\" ]]; then
    running="${BASH_REMATCH[1]}"
  fi
  [ "$running" = "$want" ] && break
  # A newer push can be installed before this one is noticed; then that one is checked.
  if [ -n "$(build "$want")" ] && [ -n "$(build "$running")" ] && [ "$(build "$running")" -gt "$(build "$want")" ]; then
    echo "a newer release, $running, is already live; checking it instead of $want"
    break
  fi
  if [ "$SECONDS" -ge "$deadline" ]; then
    echo "FAIL: $base still runs '${running:-nothing}' after $wait_minutes minutes, want $want." >&2
    echo "The server either has not looked yet or rolled back after a failed health check." >&2
    exit 1
  fi
  echo "waiting: $base runs '${running:-nothing}', want $want"
  sleep 30
done
echo "ok   $base runs $running"

# shellcheck source=/dev/null  # tools/smoke-checks.sh, uses $base
. "$(dirname "$0")/smoke-checks.sh"

# A server keeps its own copy of these; a release only matters if they still answer.
check /v1/propagation application/json
check "/v1/sat-pass?sat=ISS&lat=13.08&lng=80.27" application/json
check /v1/satellites application/json
check /v1/ip/1.1.1.1 application/json

echo "production check passed: $base runs $running"
