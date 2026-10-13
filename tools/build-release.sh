#!/usr/bin/env bash
# Builds the deployable bundle into dist/: dharun-api-linux-amd64.tar.gz, its .sha256
# and VERSION. The bundle holds the server binary and the database it reads.
#
#   bash tools/build-release.sh 1.2.3
#
# PNG logos need the resvg renderer. It is downloaded (pinned version and checksum)
# unless RESVG points at one already installed.
set -euo pipefail
cd "$(dirname "$0")/.."

version="${1:?usage: build-release.sh <version>}"
name=dharun-api
stage="dist/$name"

resvg_version=0.48.1
resvg_sha256=fa8c26495a187e592c501db15bf9e8a9fdc051d4b2b336b39703d5b59f912b9d

rm -rf dist
mkdir -p "$stage" dist/tools

resvg="${RESVG:-}"
if [ -z "$resvg" ]; then
  archive=dist/tools/resvg.tar.gz
  curl -fsSL -o "$archive" "https://github.com/linebender/resvg/releases/download/v$resvg_version/resvg-linux-x86_64.tar.gz"
  echo "$resvg_sha256  $archive" | sha256sum -c --quiet -
  tar -xzf "$archive" -C dist/tools
  resvg="$PWD/dist/tools/resvg"
fi

# The data tool runs on the build machine; the server is built for the deploy target.
go run ./cmd/apidata build -strict -resvg "$resvg" -out "$stage/api.db"
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "-s -w -X main.version=$version" -o "$stage/api" ./cmd/api
printf '%s\n' "$version" > "$stage/VERSION"

tar -C dist -czf "dist/$name-linux-amd64.tar.gz" "$name"
(cd dist && sha256sum "$name-linux-amd64.tar.gz" > "$name-linux-amd64.tar.gz.sha256")
printf '%s\n' "$version" > dist/VERSION
rm -rf "$stage" dist/tools

echo "Built dist/$name-linux-amd64.tar.gz ($version, $(du -h "dist/$name-linux-amd64.tar.gz" | cut -f1))"
