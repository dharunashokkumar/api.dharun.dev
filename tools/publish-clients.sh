#!/usr/bin/env bash
# Decides whether the client packages need publishing, and prepares them.
#
#   tools/publish-clients.sh <version>
#
# Each client carries the hash of the catalog it was generated from (CATALOG, and the last
# line of its README, which both registries keep). A client is published only when that
# hash differs from the one on the registry, so most pushes publish nothing. For each
# client that is due, this sets <version> in its manifest and writes npm=1 or pypi=1 to
# $GITHUB_OUTPUT; the workflow then publishes with trusted publishing (no tokens).
set -euo pipefail

version="${1:?usage: publish-clients.sh <version>}"
output="${GITHUB_OUTPUT:-/dev/stdout}"
hash=$(grep -o 'CATALOG = "[0-9a-f]*"' clients/js/index.js | cut -d'"' -f2)
[ -n "$hash" ] || { echo "ERROR: no catalog hash in clients/js/index.js" >&2; exit 1; }

catalog_of() { grep -o "catalog [0-9a-f]\{12\}" | head -1 | cut -d' ' -f2 || true; }

npm_hash=$(npm view india-api readme 2>/dev/null | catalog_of)
pypi_hash=$(curl -fsS --retry 3 https://pypi.org/pypi/india-api/json 2>/dev/null | jq -r '.info.description // ""' | catalog_of)
echo "catalog $hash; npm has ${npm_hash:-nothing}; PyPI has ${pypi_hash:-nothing}"

if [ "$npm_hash" != "$hash" ]; then
  (cd clients/js && npm pkg set version="$version")
  echo "npm=1" >> "$output"
fi
if [ "$pypi_hash" != "$hash" ]; then
  sed -i "s/^version = \".*\"/version = \"$version\"/" clients/python/pyproject.toml
  grep -q "^version = \"$version\"" clients/python/pyproject.toml || { echo "ERROR: could not set the Python version" >&2; exit 1; }
  echo "pypi=1" >> "$output"
fi
