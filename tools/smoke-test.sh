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

# shellcheck source=/dev/null  # tools/smoke-checks.sh, uses $base
. "$(dirname "$0")/smoke-checks.sh"

# Datasets only a server fetches are not in a release; until its first refresh their
# endpoints say so rather than fail.
for path in /v1/propagation "/v1/sat-pass?sat=ISS" /v1/satellites; do
  code="$(curl -s -o /dev/null -w '%{http_code}' "$base$path")"
  [ "$code" = 503 ] || { echo "FAIL $path: $code before any refresh, want 503" >&2; exit 1; }
  echo "ok   $path answers 503 until refreshed"
done

# Every dataset the repository holds must be in the bundle and served from its seed.
health="$(curl -fsS "$base/healthz")"
for dataset in holidays-bank holidays-central holidays-market ifsc logos pincode rto thirukkural airports disposable-email mutual-funds hsn stations constituencies phones; do
  case "$health" in
    *"\"$dataset\":{\"copy\":\"seed\""*) ;;
    *) echo "FAIL: /healthz does not serve $dataset from the seed: $health" >&2; exit 1 ;;
  esac
done
echo "ok   /healthz lists every dataset"

# The data tool in the bundle runs and reads the same seeds.
# (Captured first: with pipefail, grep -q stopping early would fail the pipe.)
status="$(cd "$dir" && ./apidata status -seed seed -live "$dir/live")"
grep -q '^rto  *seed' <<<"$status" || { echo "FAIL: apidata status does not see the seeds" >&2; exit 1; }
echo "ok   apidata status"
"$dir/resvg" --version >/dev/null || { echo "FAIL: the bundled resvg does not run" >&2; exit 1; }
echo "ok   resvg"
if ! grep -q '^modernc.org/sqlite ' "$dir/NOTICES" || ! grep -q 'SIL Open Font License' "$dir/NOTICES"; then
  echo "FAIL: the bundle has no licence notices" >&2
  exit 1
fi
echo "ok   NOTICES"
if ! grep -q 'GNU AFFERO GENERAL PUBLIC LICENSE' "$dir/LICENSE"; then
  echo "FAIL: the bundle has no LICENSE" >&2
  exit 1
fi
echo "ok   LICENSE"

version="$(cat "$dir/VERSION")"
curl -fsS "$base/healthz" | grep -q "\"version\":\"$version\"" || { echo "FAIL: /healthz does not report $version" >&2; exit 1; }
echo "smoke test passed: $version"
