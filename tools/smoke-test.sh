#!/usr/bin/env bash
# Starts a built bundle and checks the routes a deploy must not break.
#   bash tools/smoke-test.sh dist/logos-api.tar.gz
set -euo pipefail

bundle="${1:?usage: smoke-test.sh <logos-api.tar.gz>}"
port="${SMOKE_PORT:-18080}"
base="http://127.0.0.1:$port"
dir="$(mktemp -d)"

tar -xzf "$bundle" -C "$dir" --strip-components=1
(cd "$dir" && PORT="$port" METRICS_PORT=0 LOG_REQUESTS=0 exec node server/index.mjs) &
pid=$!
trap 'kill "$pid" 2>/dev/null || true; rm -rf "$dir"' EXIT

for _ in $(seq 1 40); do
  curl -fsS -o /dev/null "$base/healthz" 2>/dev/null && break
  sleep 0.25
done

# <path> <expected content-type prefix>
check() {
  local type
  type="$(curl -fsS -o /dev/null -w '%{content_type}' "$base$1")"
  case "$type" in
    "$2"*) echo "ok   $1" ;;
    *) echo "FAIL $1: got '$type', want '$2'" >&2; exit 1 ;;
  esac
}

check /healthz application/json
check /logos/ text/html
check /logos/v1/logo/TCS image/svg+xml
check "/logos/v1/logo/TCS?format=png&size=64" image/png
check "/logos/v1/logo/UNKNOWN?fallback=monogram&format=png" image/png
check /logos/v1/companies/RELIANCE application/json
check "/logos/v1/search?q=tata" application/json
check /logos/company/NSE:TCS text/html
check /logos/openapi.json application/json
check /logos/llms.txt text/plain

version="$(cat "$dir/VERSION")"
curl -fsS "$base/healthz" | grep -q "\"version\":\"$version\"" || { echo "FAIL: /healthz does not report $version" >&2; exit 1; }
echo "smoke test passed: $version"
