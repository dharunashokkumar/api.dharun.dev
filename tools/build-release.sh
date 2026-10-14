#!/usr/bin/env bash
# Builds the deployable bundle into dist/: dharun-api-linux-amd64.tar.gz, its .sha256
# and VERSION. The bundle holds:
#
#   api        the server
#   apidata    the data tool; on the server it refreshes datasets (apidata refresh)
#   resvg      the SVG renderer: PNG logos at build time, generated images per request
#   typst, piper/, deep-filter, onnx-run, libonnxruntime.so.*
#              programs the file tools run (tools/fetch-tools.sh); OCR, image, PDF and
#              audio conversion use programs the server installs (deploy/install.sh)
#   seed/      every dataset as built from this repository's data/
#   LICENSE    the GNU AGPL-3.0, under which api and apidata are licensed
#   NOTICES    licences of the code and fonts compiled in, and of resvg (tools/notices.sh)
#   VERSION
#
#   bash tools/build-release.sh 1.2.3
#
# resvg is downloaded by tools/fetch-resvg.sh (pinned version and checksum) unless RESVG
# points at one already there for linux/amd64; the other programs by tools/fetch-tools.sh
# into TOOLS (default dist/tools), which also builds onnx-run and so needs a C compiler.
#
# Beside the bundle, dist/ gets espeak-ng-source.tar.gz: the GPL source of the espeak-ng
# that piper/ ships, attached to every release with the bundle.
set -euo pipefail
cd "$(dirname "$0")/.."

version="${1:?usage: build-release.sh <version>}"
name=dharun-api
stage="dist/$name"

rm -rf dist
mkdir -p "$stage" dist/tools

resvg="${RESVG:-}"
if [ -z "$resvg" ]; then
  bash tools/fetch-resvg.sh dist/tools
  resvg="$PWD/dist/tools/resvg"
fi

tools="${TOOLS:-$PWD/dist/tools}"
bash tools/fetch-tools.sh "$tools"

# The data tool runs here to build the seeds; both binaries are built for the server.
go run ./cmd/apidata build -strict -resvg "$resvg" -out "$stage/seed"
ldflags="-s -w -X main.version=$version"
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "$ldflags" -o "$stage/api" ./cmd/api
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "$ldflags" -o "$stage/apidata" ./cmd/apidata
install -m 755 "$resvg" "$stage/resvg"
for program in typst deep-filter onnx-run; do
  install -m 755 "$tools/$program" "$stage/$program"
done
install -m 644 "$tools"/libonnxruntime.so.* "$stage/"
install -m 644 "$tools/onnxruntime-ThirdPartyNotices.txt" "$stage/"
cp -a "$tools/piper" "$stage/piper"
rm -rf "$stage/piper/pkgconfig"
install -m 644 LICENSE "$stage/LICENSE"
TOOLS_DIR="$tools" bash tools/notices.sh > "$stage/NOTICES"
printf '%s\n' "$version" > "$stage/VERSION"

tar -C dist -czf "dist/$name-linux-amd64.tar.gz" "$name"
(cd dist && sha256sum "$name-linux-amd64.tar.gz" > "$name-linux-amd64.tar.gz.sha256")
printf '%s\n' "$version" > dist/VERSION
cp "$tools/espeak-ng-source.tar.gz" dist/espeak-ng-source.tar.gz
rm -rf "$stage" dist/tools

echo "Built dist/$name-linux-amd64.tar.gz ($version, $(du -h "dist/$name-linux-amd64.tar.gz" | cut -f1))"
