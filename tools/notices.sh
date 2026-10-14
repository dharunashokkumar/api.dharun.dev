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

The programs in this bundle (api, apidata) are built from the source at
https://github.com/dharunashokkumar/api.dharun.dev and include the code listed below, each
under its own licence. resvg is a separate program, shipped unchanged.

EOF

notice "Go standard library and runtime (https://go.dev)" "$(go env GOROOT)/LICENSE"

# The modules actually linked into the two programs, for the platform they are built for.
modules="$(CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go list -deps \
  -f '{{with .Module}}{{if not .Main}}{{.Path}} {{.Version}} {{.Dir}}{{end}}{{end}}' \
  ./cmd/api ./cmd/apidata | sort -u)"
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
