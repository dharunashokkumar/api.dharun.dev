#!/usr/bin/env bash
# Downloads the programs the file tools run (linux/amd64) into a directory, each pinned by
# release and checksum, and builds onnx-run against ONNX Runtime:
#
#   typst        Typst, documents to PDF (Apache-2.0)
#   piper/       Piper text to speech with espeak-ng and its own ONNX Runtime (MIT; espeak-ng GPL-3.0)
#   deep-filter  DeepFilterNet noise removal (MIT or Apache-2.0)
#   onnx-run     our helper that runs one ONNX model, with libonnxruntime.so beside it (MIT)
#   espeak-ng-source.tar.gz
#                the source of the espeak-ng in piper/ (GPL-3.0), which every release carries
#
#   bash tools/fetch-tools.sh .cache/tools     # needs Go and a C compiler for onnx-run
#
# Checksums: typst and ONNX Runtime match the digests GitHub publishes for their release
# assets; the 2023 Piper and DeepFilterNet releases predate those digests and are pinned
# to the files as first downloaded on 2026-10-14. Piper's release built espeak-ng from the
# master branch of rhasspy/espeak-ng without pinning it; 0f65aa3 was master on the day
# (14 November 2023), and nothing was committed again until the 27th. Files already there
# are kept.
set -euo pipefail
dir="${1:?usage: fetch-tools.sh <dir>}"
mkdir -p "$dir"
dir="$(cd "$dir" && pwd)"
repo="$(cd "$(dirname "$0")/.." && pwd)"

fetch() { # url sha256 file
  if ! [ -f "$dir/$3" ]; then
    curl -fsSL --retry 5 --retry-all-errors -o "$dir/$3.part" "$1"
    echo "$2  $dir/$3.part" | sha256sum -c --quiet -
    mv "$dir/$3.part" "$dir/$3"
  fi
}

typst_version=0.15.1
fetch "https://github.com/typst/typst/releases/download/v$typst_version/typst-x86_64-unknown-linux-musl.tar.xz" \
  a6d077d0a95eed5a2eba715b2dae06be954f624ccbf85758a03f389ded33118c typst.tar.xz
if ! [ -x "$dir/typst" ]; then
  tar -xJf "$dir/typst.tar.xz" -C "$dir" --strip-components=1 typst-x86_64-unknown-linux-musl/typst
fi

fetch https://github.com/rhasspy/piper/releases/download/2023.11.14-2/piper_linux_x86_64.tar.gz \
  a50cb45f355b7af1f6d758c1b360717877ba0a398cc8cbe6d2a7a3a26e225992 piper.tar.gz
if ! [ -x "$dir/piper/piper" ]; then
  tar -xzf "$dir/piper.tar.gz" -C "$dir"
fi

fetch https://github.com/Rikorose/DeepFilterNet/releases/download/v0.5.6/deep-filter-0.5.6-x86_64-unknown-linux-musl \
  70775e251eee44c0f2451a1e833326cf8bcbbe304d3e7cd12851e6fce72ef7da deep-filter
chmod +x "$dir/deep-filter"

fetch https://github.com/rhasspy/espeak-ng/archive/0f65aa301e0d6bae5e172cc74197d32a6182200f.tar.gz \
  37ebbabb010200be8ec26c61b770e93f964f4503a329d7d74c239a1a40051e43 espeak-ng-source.tar.gz

ort_version=1.29.1
fetch "https://github.com/microsoft/onnxruntime/releases/download/v$ort_version/onnxruntime-linux-x64-$ort_version.tgz" \
  a28d7d65acafc06fb0f416cb409998773f5314c7eebf77caf907812831cdfc67 onnxruntime.tgz
if ! [ -f "$dir/libonnxruntime.so.$ort_version" ]; then
  tar -xzf "$dir/onnxruntime.tgz" -C "$dir" --strip-components=2 "onnxruntime-linux-x64-$ort_version/lib/libonnxruntime.so.$ort_version"
  tar -xzf "$dir/onnxruntime.tgz" -C "$dir" --strip-components=1 "onnxruntime-linux-x64-$ort_version/LICENSE" "onnxruntime-linux-x64-$ort_version/ThirdPartyNotices.txt"
  mv "$dir/LICENSE" "$dir/onnxruntime-LICENSE"
  mv "$dir/ThirdPartyNotices.txt" "$dir/onnxruntime-ThirdPartyNotices.txt"
fi
# onnx-run loads the library at run time, so building it needs only cgo.
(cd "$repo" && CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -trimpath -o "$dir/onnx-run" ./cmd/onnx-run)

echo "tools ready in $dir: typst $("$dir/typst" --version | head -1), piper, deep-filter, onnx-run with ONNX Runtime $ort_version"
