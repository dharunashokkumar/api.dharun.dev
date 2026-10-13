#!/usr/bin/env bash
# Downloads resvg, the SVG renderer (linux/amd64), into a directory, pinned by release
# and checksum. It renders PNG logos at build time and generated images per request.
#
#   bash tools/fetch-resvg.sh .cache/tools     # leaves .cache/tools/resvg
#
# One already there is kept.
set -euo pipefail
dir="${1:?usage: fetch-resvg.sh <dir>}"
mkdir -p "$dir"

resvg_version=0.48.1
resvg_sha256=fa8c26495a187e592c501db15bf9e8a9fdc051d4b2b336b39703d5b59f912b9d
if ! [ -x "$dir/resvg" ]; then
  archive="$dir/resvg.tar.gz"
  curl -fsSL -o "$archive" "https://github.com/linebender/resvg/releases/download/v$resvg_version/resvg-linux-x86_64.tar.gz"
  echo "$resvg_sha256  $archive" | sha256sum -c --quiet -
  tar -xzf "$archive" -C "$dir"
  rm -f "$archive"
fi
echo "resvg ready: $dir/resvg ($("$dir/resvg" --version))"
