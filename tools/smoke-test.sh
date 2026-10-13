#!/usr/bin/env bash
# Starts a built bundle and checks the routes a deploy must not break, against the
# full datasets.
#   bash tools/smoke-test.sh dist/dharun-api-linux-amd64.tar.gz
set -euo pipefail

bundle="${1:?usage: smoke-test.sh <bundle.tar.gz>}"
port="${SMOKE_PORT:-18080}"
base="http://127.0.0.1:$port"
dir="$(mktemp -d)"

tar -xzf "$bundle" -C "$dir" --strip-components=1
(cd "$dir" && PORT="$port" METRICS_PORT=0 LOG_REQUESTS=0 SEED_DIR=seed DATA_DIR="$dir/live" exec ./api) &
pid=$!
trap 'kill "$pid" 2>/dev/null || true; rm -rf "$dir"' EXIT

for _ in $(seq 1 40); do
  curl -fsS -o /dev/null "$base/healthz" 2>/dev/null && break
  sleep 0.25
done

# <path> <expected content-type prefix> [text the body must contain]
check() {
  local type body
  body="$(mktemp)"
  type="$(curl -fsS -o "$body" -w '%{content_type}' "$base$1")" || { echo "FAIL $1: request failed" >&2; exit 1; }
  case "$type" in
    "$2"*) ;;
    *) echo "FAIL $1: got '$type', want '$2'" >&2; exit 1 ;;
  esac
  if [ -n "${3:-}" ] && ! grep -q -- "$3" "$body"; then
    echo "FAIL $1: body does not contain '$3'" >&2
    exit 1
  fi
  rm -f "$body"
  echo "ok   $1"
}

check /healthz application/json
check / text/html "open APIs for Indian developers"
check /v1 application/json
check /openapi.json application/json
check /llms.txt text/plain
check /v1/logos/TCS image/svg+xml
check "/v1/logos/TCS?format=png&size=64" image/png
check "/v1/logos/UNKNOWN?fallback=monogram&format=png" image/png
check /v1/companies/RELIANCE application/json '"NSE:RELIANCE"'
check /v1/pincode/637001 application/json '"Namakkal"'
check /v1/ifsc/SBIN0000001 application/json '"State Bank of India"'
check /v1/rto/TN-28 application/json 'Namakkal'
check "/v1/holidays/2026?type=market" application/json 'Republic Day'
check /v1/words/100000 application/json '"one lakh"'
check /v1/validate/gstin/27AAPFU0939F1ZV application/json '"valid":true'
check /v1/states/TN application/json '"Tamil Nadu"'
check "/v1/pincode/near?lat=11.2189&lng=78.1674" application/json '"distanceKm"'
check "/v1/working-days/2026-01-16?calendar=bank&state=TN" application/json '"working":false'
check "/v1/working-days/2026-02-18?calendar=settlement&add=1" application/json '"shifted":"2026-02-20"'
check "/v1/market/status?at=2026-01-26T10:00:00%2B05:30" application/json 'Republic Day'
check "/v1/qr?text=hello&format=png" image/png
check "/v1/upi-qr?pa=shop@okaxis&am=250&format=json" application/json 'upi://pay?pa=shop@okaxis'
check "/v1/barcode/code128?text=AWB1234" image/svg+xml
check /v1/thirukkural/1 application/json '"The Praise of God"'
check /v1/airports/MAA application/json '"VOMM"'
check /v1/mutual-funds/122639 application/json '"PPFAS Mutual Fund"'
check "/v1/text/tokens?text=hello%20world" application/json '"tokens":2'
check "/v1/text/redact?text=PAN%20ABCPK1234F" application/json '[PAN]'
check "/v1/transliterate?to=ascii&text=%E0%AE%A4%E0%AE%AE%E0%AE%BF%E0%AE%B4%E0%AF%8D" application/json '"tamizh"'

# Every dataset the repository holds must be in the bundle and served from its seed.
health="$(curl -fsS "$base/healthz")"
for dataset in holidays-bank holidays-central holidays-market ifsc logos pincode rto thirukkural airports disposable-email mutual-funds; do
  case "$health" in
    *"\"$dataset\":{\"copy\":\"seed\""*) ;;
    *) echo "FAIL: /healthz does not serve $dataset from the seed: $health" >&2; exit 1 ;;
  esac
done
echo "ok   /healthz lists every dataset"

# The data tool in the bundle runs and reads the same seeds.
(cd "$dir" && ./apidata status -seed seed -live "$dir/live") | grep -q '^rto  *seed' ||
  { echo "FAIL: apidata status does not see the seeds" >&2; exit 1; }
echo "ok   apidata status"
"$dir/resvg" --version >/dev/null || { echo "FAIL: the bundled resvg does not run" >&2; exit 1; }
echo "ok   resvg"

version="$(cat "$dir/VERSION")"
curl -fsS "$base/healthz" | grep -q "\"version\":\"$version\"" || { echo "FAIL: /healthz does not report $version" >&2; exit 1; }
echo "smoke test passed: $version"
