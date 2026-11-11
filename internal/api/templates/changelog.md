{{define "changelog"}}# Changelog

What changed for the people and programs calling India API, newest first. The code's full history is in the [repository]({{.Repo}}/commits/main).

## November 2026

- **Free API keys.** [Sign up]({{.Origin}}/signup) with just your email address (we mail you a code; no passwords) for a key: {{.KeyLimit}} units a minute per key instead of {{.Limit}} per address, on every endpoint. The model endpoints (OCR, speech to text, text to speech, translation, embeddings, reranking, moderation, Roman typing, background removal, denoise, and name detection in redaction) will need a key after a notice period; until then they answer with an `X-Key-Notice` header. See [API keys]({{.Origin}}/keys).
- **Rate-limit headers** on every charged answer: `RateLimit-Limit`, `RateLimit-Remaining`, `RateLimit-Reset`.
- **Try it on every API page**, a [playground]({{.Origin}}/play), [free tools]({{.Origin}}/tools), [demos]({{.Origin}}/demos) and [company logo pages]({{.Origin}}/logos).
- **New endpoints:** batch lookups (`POST /v1/batch`), calendar feeds (`/v1/holidays/{year}.ics`), today in India (`/v1/today`), money calculators (`/v1/calc/...`: SIP, lump sum, EMI, GST, income tax, HRA, gratuity, PPF, capital gains, trade charges), avatars (`/v1/avatar/{name}`), placeholder images (`/v1/placeholder/{size}`) and chart images (`/v1/chart`).
- **Logos never fail.** An unknown name answers with an initials tile (status 200, `X-Logo-Known: false`) instead of 404; `fallback=none` keeps the 404. New logo sets for brands, global stocks and crypto, indices, ETFs and commodities; `/v1/logos` falls through to them. Cached logo answers no longer count against the rate limit.

## October 2026

- Labels for attacks and errors in the log and metrics; the panchang's search for the next phase fixed.
- AI models on the server: embeddings, reranking, moderation, name detection, translation between English and 22 Indian languages, Roman typing, speech to text.
- File tools: OCR, image conversion and background removal, PDF tools, text to speech, audio denoise, GST invoices as PDF.
{{end}}
