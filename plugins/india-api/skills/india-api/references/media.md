# Audio and video tools API

> Convert, trim, draw a waveform or take a frame, with ffmpeg.

Files in, files out, nothing kept. Each counts as 30 against the rate limit. Needs a free API key once keys are required (see /keys).

Base URL `https://api.dharun.dev`. No API key needed for most endpoints; a free key (https://api.dharun.dev/keys) raises the limits, and the model endpoints marked below will need one. Answers are JSON, `{ "data": ..., "meta": { ... } }`, and errors are `{ "error": { "code", "message" } }` with a matching HTTP status. Rate limit: 600 units a minute per address without a key, 3000 per key; the `RateLimit-*` headers say what is left.

## Convert audio or video

`POST /v1/media/convert`

To MP3, WAV, OGG (Opus), M4A, FLAC, MP4, WebM or an animated GIF. Options go in the query string, or as form fields when the file is sent as multipart/form-data.

Send: Audio or video ffmpeg can read (MP3, WAV, M4A, OGG, FLAC, MP4, MOV, WebM, MKV...), up to 10 MB; the first 10 minutes are used. Post the file as the request body, or as `multipart/form-data`. Sample file: https://api.dharun.dev/samples/speech.mp3

Model endpoint: it will need a free API key (sent as `Authorization: Bearer <key>` or `X-API-Key`). Until the date on https://api.dharun.dev/keys it answers without one and says so in the `X-Key-Notice` header. With a key: 500 model requests a day.

What you send is never written to the log, and the answer is not cached.

Parameters:

- `format` (query, required): What to make. One of: mp3, wav, ogg, m4a, flac, mp4, webm, gif.

Example: `POST` https://api.dharun.dev/v1/media/convert?format=wav&utm_medium=skill (returns the audio itself)

## Cut a clip

`POST /v1/media/trim`

Keeps the part from start to end. Options go in the query string, or as form fields when the file is sent as multipart/form-data.

Send: Audio or video ffmpeg can read (MP3, WAV, M4A, OGG, FLAC, MP4, MOV, WebM, MKV...), up to 10 MB; the first 10 minutes are used. Post the file as the request body, or as `multipart/form-data`. Sample file: https://api.dharun.dev/samples/speech.mp3

Model endpoint: it will need a free API key (sent as `Authorization: Bearer <key>` or `X-API-Key`). Until the date on https://api.dharun.dev/keys it answers without one and says so in the `X-Key-Notice` header. With a key: 500 model requests a day.

What you send is never written to the log, and the answer is not cached.

Parameters:

- `start` (query): Seconds, or m:ss. Default 0.
- `end` (query, required): Seconds, or m:ss.
- `format` (query): Output format when the file's own is not one of these. One of: mp3, wav, ogg, m4a, flac, mp4, webm, gif.

Example: `POST` https://api.dharun.dev/v1/media/trim?start=0&end=1.5&utm_medium=skill (returns the audio itself)

## Waveform picture

`POST /v1/media/waveform`

Send: Audio or video ffmpeg can read (MP3, WAV, M4A, OGG, FLAC, MP4, MOV, WebM, MKV...), up to 10 MB; the first 10 minutes are used. Post the file as the request body, or as `multipart/form-data`. Sample file: https://api.dharun.dev/samples/speech.mp3

Model endpoint: it will need a free API key (sent as `Authorization: Bearer <key>` or `X-API-Key`). Until the date on https://api.dharun.dev/keys it answers without one and says so in the `X-Key-Notice` header. With a key: 500 model requests a day.

What you send is never written to the log, and the answer is not cached.

Parameters:

- `width` (query): 100 to 4000. Default 1200.
- `height` (query): 40 to 1000. Default 200.
- `color` (query): Hex. Default 2563eb.

Example: `POST` https://api.dharun.dev/v1/media/waveform?width=800&height=160&utm_medium=skill (returns the image itself)

## A frame from a video

`POST /v1/media/thumbnail`

Send: Audio or video ffmpeg can read (MP3, WAV, M4A, OGG, FLAC, MP4, MOV, WebM, MKV...), up to 10 MB; the first 10 minutes are used. Post the file as the request body, or as `multipart/form-data`. Sample file: https://api.dharun.dev/samples/clip.mp4

Model endpoint: it will need a free API key (sent as `Authorization: Bearer <key>` or `X-API-Key`). Until the date on https://api.dharun.dev/keys it answers without one and says so in the `X-Key-Notice` header. With a key: 500 model requests a day.

What you send is never written to the log, and the answer is not cached.

Parameters:

- `at` (query): Time, seconds or m:ss. Default 1.
- `width` (query): 32 to 3840. Default 640.

Example: `POST` https://api.dharun.dev/v1/media/thumbnail?at=1&utm_medium=skill (returns the image itself)
