#!/usr/bin/env bash
# Writes the licence notices for everything compiled into or shipped with the release:
# the Go standard library, every module linked into api and apidata, code copied into
# this repository under its own licence, the fonts, and resvg.
#
#   bash tools/notices.sh > NOTICES
#
# A linked module without a licence file stops the build: a new dependency gets its
# notice, or the release does not go out.
set -euo pipefail
cd "$(dirname "$0")/.."

notice() { # title, file
  printf '%s\n%s\n\n' "================================================================================" "$1"
  cat "$2"
  printf '\n'
}

cat <<'EOF'
Third-party notices for dharun-api

The programs in this bundle built here (api, apidata, onnx-run) are free software under
the GNU Affero General Public License v3.0 (AGPL-3.0-only, in LICENSE), built from the
source at https://github.com/dharunashokkumar/api.dharun.dev. They include the code
listed below, each under its own licence. resvg, typst, piper, deep-filter and ONNX
Runtime are separate programs, shipped unchanged; their licences follow at the end.

EOF

notice "Go standard library and runtime (https://go.dev)" "$(go env GOROOT)/LICENSE"

# The modules actually linked into the programs, for the platform they are built for.
# onnx-run is built with cgo, so its modules are listed with cgo on.
modules="$( (CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go list -deps \
  -f '{{with .Module}}{{if not .Main}}{{.Path}} {{.Version}} {{.Dir}}{{end}}{{end}}' \
  ./cmd/api ./cmd/apidata
  CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go list -deps \
  -f '{{with .Module}}{{if not .Main}}{{.Path}} {{.Version}} {{.Dir}}{{end}}{{end}}' \
  ./cmd/onnx-run) | sort -u)"
missing=0
while read -r path version dir; do
  found=0
  for f in "$dir"/LICENSE* "$dir"/LICENCE* "$dir"/COPYING* "$dir"/NOTICE* "$dir"/UNLICENSE*; do
    [ -f "$f" ] || continue
    notice "$path $version ($(basename "$f"))" "$f"
    found=1
  done
  if [ "$found" = 0 ]; then
    echo "notices.sh: no licence file in $path $version ($dir)" >&2
    missing=1
  fi
done <<<"$modules"
[ "$missing" = 0 ] || exit 1

notice "SGP4 from github.com/joshuaferrara/go-satellite, copied into internal/orbit" internal/orbit/LICENSE.go-satellite
notice "Noto fonts (https://github.com/notofonts), compiled into api" internal/typeset/fonts/LICENSE

cat <<'EOF'
================================================================================
resvg (https://github.com/linebender/resvg), shipped as the separate program resvg

resvg is licensed under the Apache License 2.0 or the MIT licence, at your option.
Its source, licence texts and the licences of the Rust crates it is built from are
published with each release at https://github.com/linebender/resvg/releases.
EOF

cat <<'EOF'

================================================================================
Typst 0.15.1 (https://github.com/typst/typst), shipped as the separate program typst

Typst is licensed under the Apache License 2.0. Its source and the licences of the crates
and fonts it is built from are at https://github.com/typst/typst/releases/tag/v0.15.1.

================================================================================
Piper 2023.11.14-2 (https://github.com/rhasspy/piper), shipped unchanged in piper/

Piper is licensed under the MIT licence. Its folder holds libraries it was released with:
- espeak-ng (libespeak-ng, espeak-ng-data), licensed under the GNU General Public
  License v3.0 or later. Its source, commit 0f65aa3 of https://github.com/rhasspy/espeak-ng
  (the fork Piper's release was built from), is attached to every release of this
  project as espeak-ng-source.tar.gz, beside this bundle.
- piper-phonemize and its tashkeel model (MIT): https://github.com/rhasspy/piper-phonemize
- ONNX Runtime 1.14.1 (MIT): https://github.com/microsoft/onnxruntime
The voices are not part of this bundle; each names its own licence in the API.

================================================================================
DeepFilterNet 0.5.6 (https://github.com/Rikorose/DeepFilterNet), shipped as deep-filter

DeepFilterNet is licensed under the MIT licence or the Apache License 2.0, at your
option. Source and licences: https://github.com/Rikorose/DeepFilterNet/releases/tag/v0.5.6

================================================================================
ONNX Runtime 1.29.1 (https://github.com/microsoft/onnxruntime), libonnxruntime.so, loaded by onnx-run

EOF
cat "${TOOLS_DIR:?TOOLS_DIR names the folder tools/fetch-tools.sh filled}/onnxruntime-LICENSE"
printf '\nIts third-party notices are in onnxruntime-ThirdPartyNotices.txt, beside this file.\n'
