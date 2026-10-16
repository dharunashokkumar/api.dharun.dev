# shellcheck shell=bash
# The routes a deploy must not break, with what each must answer. Sourced with $base set to
# the server to check: by smoke-test.sh against a fresh bundle on the full datasets, and by
# prod-check.sh against the public server once a release is live.
# shellcheck disable=SC2154  # base comes from the script that sources this
# Requests from these checks name themselves, so the server counts them as its own traffic.
check_agent="india-api-check/1 (+https://github.com/dharunashokkumar/api.dharun.dev)"
# <path> <expected content-type prefix> [text the body must contain]
check() {
  local type body
  body="$(mktemp)"
  type="$(curl -fsS -A "$check_agent" -m 30 --retry 2 --retry-delay 3 -o "$body" -w '%{content_type}' "$base$1")" || { echo "FAIL $1: request failed" >&2; exit 1; }
  case "$type" in
    "$2"*) ;;
    *) echo "FAIL $1: got '$type', want '$2'" >&2; exit 1 ;;
  esac
  if [ -n "${3:-}" ] && ! grep -q -- "$3" "$body"; then
    echo "FAIL $1: body does not contain '$3'" >&2
    exit 1
  fi
  rm -f "$body"
  echo "ok   $1"
}

# <path> <file to post, from the sample files> <content type it is sent as> <expected content-type prefix> [text]
post_check() {
  local type body sample
  body="$(mktemp)"
  sample="$(mktemp)"
  curl -fsS -A "$check_agent" -m 30 --retry 2 -o "$sample" "$base/samples/$2" || { echo "FAIL $1: no sample $2" >&2; exit 1; }
  type="$(curl -fsS -A "$check_agent" -m 60 --retry 2 --retry-delay 3 -H "Content-Type: $3" --data-binary "@$sample" -o "$body" -w '%{content_type}' "$base$1")" || { echo "FAIL POST $1: request failed" >&2; exit 1; }
  case "$type" in
    "$4"*) ;;
    *) echo "FAIL POST $1: got '$type', want '$4'" >&2; exit 1 ;;
  esac
  if [ -n "${5:-}" ] && ! grep -q -- "$5" "$body"; then
    echo "FAIL POST $1: body does not contain '$5'" >&2
    exit 1
  fi
  rm -f "$body" "$sample"
  echo "ok   POST $1"
}

# <path> <JSON body> <expected content-type prefix> [text the answer must contain]
post_json() {
  local type body
  body="$(mktemp)"
  type="$(curl -fsS -A "$check_agent" -m 60 --retry 2 --retry-delay 3 -H "Content-Type: application/json" --data-binary "$2" -o "$body" -w '%{content_type}' "$base$1")" || { echo "FAIL POST $1: request failed" >&2; exit 1; }
  case "$type" in
    "$3"*) ;;
    *) echo "FAIL POST $1: got '$type', want '$3'" >&2; exit 1 ;;
  esac
  if [ -n "${4:-}" ] && ! grep -q -- "$4" "$body"; then
    echo "FAIL POST $1: body does not contain '$4'" >&2
    exit 1
  fi
  rm -f "$body"
  echo "ok   POST $1"
}

check /healthz application/json
check / text/html "Free APIs for India."
check /docs text/html "API reference"
check /docs/pincode text/html "Department of Posts"
check /docs/pincode.md text/markdown '"district"'
check /docs.md text/markdown '/v1/pincode/{pincode}'
check /guides.md text/markdown '/guides/pincode-autofill.md'
check /guides/pincode-autofill text/html "MIT-0"
check /ai text/html "india-api"
check /privacy text/html "never logged"
check /llms-full.txt text/plain "# Pincodes API"
check /sitemap.xml application/xml "/guides/"
check /robots.txt text/plain "Content-Signal"
check /.well-known/security.txt text/plain "Contact: mailto:me@dharun.dev"
check /mcp/server-card application/mcp-server-card+json "io.github.dharunashokkumar/india-api"
check /.well-known/agent-skills/index.json application/json "sha256:"
check /v1 application/json
check /openapi.json application/json
check /llms.txt text/plain
check /v1/logos/TCS image/svg+xml
check "/v1/logos/TCS?format=png&size=64" image/png
check "/v1/logos/UNKNOWN?fallback=monogram&format=png" image/png
check /v1/companies/RELIANCE application/json '"NSE:RELIANCE"'
check /v1/pincode/637001 application/json '"Namakkal"'
check /v1/ifsc/SBIN0000001 application/json '"State Bank of India"'
check /v1/banks/HDFC application/json '"HDFC Bank"'
check /v1/rto/TN-28 application/json 'Namakkal'
check "/v1/holidays/2026?type=market" application/json 'Republic Day'
check /v1/words/100000 application/json '"one lakh"'
check /v1/validate/gstin/27AAPFU0939F1ZV application/json '"valid":true'
check /v1/states/TN application/json '"Tamil Nadu"'
check "/v1/pincode/near?lat=11.2189&lng=78.1674" application/json '"distanceKm"'
check "/v1/working-days/2026-01-16?calendar=bank&state=TN" application/json '"working":false'
check "/v1/working-days/2026-02-18?calendar=settlement&add=1" application/json '"shifted":"2026-02-20"'
check "/v1/working-days/2026-03-04?state=JK&office=Srinagar" application/json '"working":true'
check "/v1/market/status?at=2026-01-26T10:00:00%2B05:30" application/json 'Republic Day'
check "/v1/qr?text=hello&format=png" image/png
check "/v1/upi-qr?pa=shop@okaxis&am=250&format=json" application/json 'upi://pay?pa=shop@okaxis'
check "/v1/barcode/code128?text=AWB1234" image/svg+xml
check /v1/thirukkural/1 application/json '"The Praise of God"'
check /v1/airports/MAA application/json '"VOMM"'
check /v1/mutual-funds/122639 application/json '"PPFAS Mutual Fund"'
check /v1/hsn/0902 application/json '"chapter"'
check /v1/stations/MAS application/json '"Tamil Nadu"'
check /v1/stations/zones application/json '"Southern"'
check "/v1/panchang/2026-01-14?pincode=600001" application/json '"Thai"'
check "/v1/constituency?pincode=637001" application/json '"Namakkal"'
check /v1/phone/9500112345 application/json '"mobile"'
check "/v1/fake/companies?count=2&state=TN&seed=1" application/json '"gstin":"33'
check "/v1/text/tokens?text=hello%20world" application/json '"tokens":2'
check "/v1/text/redact?text=PAN%20ABCPK1234F" application/json '[PAN]'
check "/v1/text/chunk?size=100&text=First%20paragraph.%0A%0ASecond%20paragraph." application/json '"chunks"'
check "/v1/text/language?text=%E0%AE%B5%E0%AE%A3%E0%AE%95%E0%AF%8D%E0%AE%95%E0%AE%AE%E0%AF%8D%20%E0%AE%A8%E0%AE%A3%E0%AF%8D%E0%AE%AA%E0%AE%B0%E0%AF%8D%E0%AE%95%E0%AE%B3%E0%AF%87" application/json '"name":"Tamil"'
check /v1/tamil-font/encodings application/json '"tscii"'
check "/v1/tamil-font/convert?from=tscii&text=%C2%BE%C3%81%C2%A2%C3%BA" application/json '"text":"தமிழ்"'
check "/v1/og-image?title=%E0%AE%B5%E0%AE%A3%E0%AE%95%E0%AF%8D%E0%AE%95%E0%AE%AE%E0%AF%8D&site=api.dharun.dev" image/png
check "/v1/og-image?title=Hello&format=svg" image/svg+xml '<use '
check "/v1/code-image?language=go&code=package%20main" image/png
check "/v1/transliterate?to=ascii&text=%E0%AE%A4%E0%AE%AE%E0%AE%BF%E0%AE%B4%E0%AF%8D" application/json '"tamizh"'
check "/v1/sun-moon/2026-10-14?pincode=600001" application/json '"waxing crescent"'
check "/v1/regex?pattern=%5Cd%2B&text=a1b22" application/json '"count":2'
check /docs/invoice text/html "CGST"
# The network tools ask other sites, so only their pages are checked here: a site that is
# down must not stop a release.
check /docs/whois text/html "RDAP"
check /docs/probe.md text/markdown "IndiaAPI-check"
check /samples/document.pdf application/pdf
check /v1/tts/voices application/json '"hi_IN-rohan-medium"'
# The AI models are fetched by each server, so a fresh bundle answers their endpoints 503;
# only the language list, which needs no model, is checked.
check /v1/translate/languages application/json '"tam_Taml"'
check "/v1/typst?text=%3D%20%E0%AE%B5%E0%AE%A3%E0%AE%95%E0%AF%8D%E0%AE%95%E0%AE%AE%E0%AF%8D" application/pdf
post_check /v1/pdf/info document.pdf application/pdf application/json '"pages":2'
post_check /v1/barcode/read codes.png image/png application/json '"ean13"'
post_check /v1/pdf/text document.pdf application/pdf application/json 'India API sample document'
post_check /v1/ocr receipt.png image/png application/json 'MURUGAN'
post_check /v1/image/info photo.jpg image/jpeg application/json '"width":853'
post_check "/v1/image/convert?format=webp&width=200" photo.jpg image/jpeg image/webp
post_json "/v1/invoice?format=json" '{"number":"T-1","date":"2026-10-14","seller":{"name":"A","address":"Namakkal","gstin":"33AABCS1429B1Z1"},"buyer":{"name":"B","address":"Bengaluru","gstin":"29AAACK6812C1ZS"},"items":[{"description":"Rice","hsn":"1006","rate":100,"gst_rate":5}]}' application/json '"igst":5'
post_json /v1/invoice '{"number":"T-1","date":"2026-10-14","seller":{"name":"A","address":"Namakkal","gstin":"33AABCS1429B1Z1"},"buyer":{"name":"B","address":"Bengaluru","gstin":"29AAACK6812C1ZS"},"items":[{"description":"Rice","hsn":"1006","rate":100,"gst_rate":5}]}' application/pdf
