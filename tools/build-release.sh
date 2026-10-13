#!/usr/bin/env bash
# Builds the deployable bundle into dist/: logos-api.tar.gz, its .sha256 and VERSION.
# Build on the platform you deploy to (CI uses Linux x64): the PNG renderer is a native module.
set -euo pipefail
cd "$(dirname "$0")/.."

version="${1:-$(node -p "require('./package.json').version")}"
stage=dist/logos-api

rm -rf dist
mkdir -p "$stage/data"
cp -r server web logos package.json package-lock.json "$stage/"
cp data/logos.json "$stage/data/"
printf '%s\n' "$version" > "$stage/VERSION"
(cd "$stage" && npm ci --omit=dev --no-audit --no-fund)

tar -C dist -czf dist/logos-api.tar.gz logos-api
(cd dist && sha256sum logos-api.tar.gz > logos-api.tar.gz.sha256)
printf '%s\n' "$version" > dist/VERSION
rm -rf "$stage"

echo "Built dist/logos-api.tar.gz ($version, $(du -h dist/logos-api.tar.gz | cut -f1))"
