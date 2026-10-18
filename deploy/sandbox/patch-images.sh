#!/usr/bin/env bash
# Puts a new sandbox-agent (and sandbox-browser) into built images without rebuilding
# them. Each image is patched in a copy that then replaces it, so microVMs running on the
# old file keep it until they end.
#
#   patch-images.sh <images dir> <bin dir>
set -euo pipefail

images="${1:?usage: patch-images.sh <images dir> <bin dir>}"
bin="${2:?usage: patch-images.sh <images dir> <bin dir>}"

# put copies a file into an image as root:root with the given mode.
put() { # image source dir name
  local image="$1" source="$2" dir="$3" name="$4"
  debugfs -w -f - "$image" >/dev/null <<EOF
cd $dir
rm $name
write $source $name
sif $name mode 0100755
sif $name uid 0
sif $name gid 0
EOF
}

for image in "$images"/*.ext4; do
  name=$(basename "$image" .ext4)
  [ "$name" = scratch ] && continue
  cp --sparse=always --reflink=auto "$image" "$image.patch"
  put "$image.patch" "$bin/sandbox-agent" / sandbox-agent
  if [ "$name" = browser ]; then
    put "$image.patch" "$bin/sandbox-browser" /usr/local/bin sandbox-browser
  fi
  e2fsck -fn "$image.patch" >/dev/null 2>&1 || { echo "ERROR: $name.ext4 failed its check after patching" >&2; rm -f "$image.patch"; exit 1; }
  mv "$image.patch" "$image"
  echo "patch-images: $name"
done
