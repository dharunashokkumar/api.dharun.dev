#!/usr/bin/env bash
# Downloads the fonts that generated images are drawn with into internal/typeset/fonts,
# where they are compiled into the server: Noto Sans for Latin and nine Indian scripts,
# and Noto Sans Mono, each in regular and bold, with their licence.
#
#   bash tools/fetch-fonts.sh
#
# The fonts are committed, so this is only for changing them: pin a new commit below,
# run it with FONTS_UPDATE=1 to rewrite tools/fonts.sha256, and commit both. Without
# that, every file is checked against tools/fonts.sha256.
# Noto fonts are licensed under the SIL Open Font License 1.1.
set -euo pipefail
cd "$(dirname "$0")/.."
dir=internal/typeset/fonts
sums=tools/fonts.sha256
mkdir -p "$dir"

commit=4e15001b9fb484cc871d7b2ad930cd2226edf260
base="https://raw.githubusercontent.com/notofonts/notofonts.github.io/$commit/fonts"

fresh="$(mktemp)"
trap 'rm -f "$fresh"' EXIT
while read -r _ name; do
  [ -n "$name" ] || continue
  if [ "$name" = LICENSE ]; then
    curl -fsSL -o "$dir/$name" "$base/LICENSE"
  else
    curl -fsSL -o "$dir/$name" "$base/${name%%-*}/hinted/ttf/$name"
  fi
  (cd "$dir" && sha256sum "$name") | sed 's/  / /' >> "$fresh"
done < "$sums"

if [ "${FONTS_UPDATE:-}" = 1 ]; then
  cp "$fresh" "$sums"
  echo "rewrote $sums"
elif ! diff -u "$sums" "$fresh"; then
  echo "the downloaded fonts do not match $sums" >&2
  exit 1
fi
echo "fonts ready in $dir ($(find "$dir" -name '*.ttf' | wc -l) fonts)"
