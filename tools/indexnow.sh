#!/usr/bin/env bash
# Tells IndexNow search engines (Bing, Yandex, Seznam, Naver and others) which pages of
# the live site changed, so new and updated pages are crawled within hours.
#
#   bash tools/indexnow.sh https://api.dharun.dev KEY STATE_FILE
#
# STATE_FILE holds the hash of every page from the last run (CI keeps it in its cache).
# Only pages whose content changed since then are submitted; the first run submits all.
# The key is public by design: the site serves it at /<key>.txt to prove ownership.
set -euo pipefail

origin=${1:?origin}
key=${2:?key}
state=${3:?state file}
host=${origin#https://}

touch "$state"
next=$(mktemp)
changed=$(mktemp)
trap 'rm -f "$next" "$changed"' EXIT

# The site has thousands of pages (one per logo) and each takes about a second to reach
# from outside, so pages are fetched 48 at a time. Pages cost nothing against the rate limit.
agent="india-api-check/1 (+https://github.com/dharunashokkumar/api.dharun.dev)"
export agent
# shellcheck disable=SC2016  # $1 and $agent expand in the child shell
curl -fsS -A "$agent" "$origin/sitemap.xml" | grep -o '<loc>[^<]*</loc>' | sed 's/<loc>//; s/<\/loc>//' |
  xargs -r -P 48 -I{} bash -c 'printf "%s %s\n" "$1" "$(curl -fsS --retry 2 -A "$agent" "$1" | sha256sum | cut -d" " -f1)"' _ {} >"$next"
sort -o "$next" "$next"
# A page is changed when its line (address and hash) is not in the last run's state.
grep -vxFf "$state" "$next" | cut -d' ' -f1 >"$changed" || true

count=$(wc -l <"$changed")
if [ "$count" -eq 0 ]; then
  echo "No page changed; nothing to submit."
  mv "$next" "$state"
  exit 0
fi

body=$(jq -n --arg host "$host" --arg key "$key" --arg location "$origin/$key.txt" \
  --rawfile urls "$changed" \
  '{host: $host, key: $key, keyLocation: $location, urlList: ($urls | split("\n") | map(select(length > 0)))}')
status=$(curl -sS -o /dev/null -w '%{http_code}' -X POST https://api.indexnow.org/indexnow \
  -H 'Content-Type: application/json; charset=utf-8' --data "$body")
echo "Submitted $count changed pages to IndexNow: HTTP $status"
case "$status" in
  200 | 202) mv "$next" "$state" ;;
  *)
    echo "WARNING: IndexNow did not accept the submission; it will be retried next release." >&2
    exit 1
    ;;
esac
