#!/usr/bin/env bash
# Builds the deployable bundle into dist/: dharun-api-linux-amd64.tar.gz, its .sha256
# and VERSION. The bundle holds:
#
#   api        the server
#   apidata    the data tool; on the server it refreshes datasets (apidata refresh)
#   resvg      the SVG renderer, for PNG output at request time
#   seed/      every dataset as built from this repository's data/
#   VERSION
#
#   bash tools/build-release.sh 1.2.3
#
# resvg is downloaded (pinned version and checksum) unless RESVG points at one already
# installed for linux/amd64.
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

# The data tool runs here to build the seeds; both binaries are built for the server.
go run ./cmd/apidata build -strict -resvg "$resvg" -out "$stage/seed"
ldflags="-s -w -X main.version=$version"
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "$ldflags" -o "$stage/api" ./cmd/api
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "$ldflags" -o "$stage/apidata" ./cmd/apidata
install -m 755 "$resvg" "$stage/resvg"
printf '%s\n' "$version" > "$stage/VERSION"

tar -C dist -czf "dist/$name-linux-amd64.tar.gz" "$name"
(cd dist && sha256sum "$name-linux-amd64.tar.gz" > "$name-linux-amd64.tar.gz.sha256")
printf '%s\n' "$version" > dist/VERSION
rm -rf "$stage" dist/tools

echo "Built dist/$name-linux-amd64.tar.gz ($version, $(du -h "dist/$name-linux-amd64.tar.gz" | cut -f1))"
