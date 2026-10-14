#!/usr/bin/env bash
# Makes the sample files in internal/api/samples/, which the documentation's examples and
# the tests post. Every one is made here, so they are ours to share (MIT-0):
#
#   document.pdf  two A4 pages set by Typst, English, Tamil and Hindi
#   receipt.png   a shop receipt in English and Tamil, for OCR
#   codes.png     a QR code and an EAN-13 barcode drawn by this API
#   photo.jpg     a picture with one subject on a plain background, drawn by Typst
#   noisy.mp3     a sentence spoken by Piper's public-domain LJ Speech voice, with noise and hum
#
# Needs typst, piper (with the en_US-ljspeech-medium voice in $VOICES), ffmpeg, vips and Go:
#   VOICES=/path/to/voices bash tools/samples/make-samples.sh
set -euo pipefail
cd "$(dirname "$0")/../.."
out=internal/api/samples
work=$(mktemp -d)
trap 'rm -rf "$work"; kill "${server:-0}" 2>/dev/null || true' EXIT
fonts=internal/typeset/fonts
compile() { typst compile --font-path "$fonts" --ignore-system-fonts "$@"; }

# Fixed dates, so the files only change when this script does.
export SOURCE_DATE_EPOCH=1792022400 # 2026-10-14
compile --creation-timestamp "$SOURCE_DATE_EPOCH" tools/samples/document.typ "$out/document.pdf"
compile --format png --ppi 200 tools/samples/receipt.typ "$work/receipt.png"
vips copy "$work/receipt.png" "$out/receipt.png[compression=9,keep=none]"
compile --format png --ppi 96 tools/samples/photo.typ "$work/photo.png"
vips copy "$work/photo.png" "$out/photo.jpg[Q=85,keep=none,optimize_coding]"

# The codes come from this API itself.
mkdir -p "$work/seed"
go build -o "$work/api" ./cmd/api
SEED_DIR="$work/seed" PORT=18080 METRICS_PORT=0 LOG_REQUESTS=false "$work/api" >/dev/null &
server=$!
for _ in $(seq 50); do curl -fs http://127.0.0.1:18080/healthz >/dev/null && break; sleep 0.2; done
curl -fs "http://127.0.0.1:18080/v1/qr?text=https://api.dharun.dev&format=png&size=360" -o "$work/qr.png"
curl -fs "http://127.0.0.1:18080/v1/barcode/ean13?text=890123456789&format=png&scale=3&height=150" -o "$work/ean.png"
cp tools/samples/codes.typ "$work/"
compile --format png --ppi 96 --root "$work" "$work/codes.typ" "$work/codes-big.png"
vips copy "$work/codes-big.png" "$out/codes.png[compression=9,keep=none]"

piper --model "$VOICES/en_US-ljspeech-medium.onnx" --output_file "$work/voice.wav" --quiet <<<'This is a sample recording for the India API noise removal example. The fan and the hum behind me should disappear.'
ffmpeg -hide_banner -loglevel error -y -i "$work/voice.wav" \
  -f lavfi -i "anoisesrc=color=pink:amplitude=0.05:seed=7" -f lavfi -i "sine=frequency=100:sample_rate=22050" \
  -filter_complex "[2]volume=0.04[hum];[0][1][hum]amix=inputs=3:duration=first:normalize=0" \
  -ac 1 -ar 22050 -c:a libmp3lame -b:a 48k -map_metadata -1 "$out/noisy.mp3"

ls -la "$out"
