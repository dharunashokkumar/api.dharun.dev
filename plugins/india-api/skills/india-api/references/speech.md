# Speech API

> Text to speech in Hindi, Telugu, Malayalam and English, and noise removal for recordings.

Turn text into natural speech with Piper voices, as MP3, Ogg Opus or WAV, or clean background noise from a recording with DeepFilterNet. Each voice keeps the licence of the recordings it learnt from: /v1/tts/voices lists them, and two Hindi voices are for non-commercial use only. There is no Tamil, Kannada, Bengali, Gujarati or Marathi voice yet. Nothing you send is kept or logged.

Base URL `https://api.dharun.dev`. No API key. Answers are JSON, `{ "data": ..., "meta": { ... } }`, and errors are `{ "error": { "code", "message" } }` with a matching HTTP status. Rate limit: 600 requests a minute per client.

## Text to speech

`GET /v1/tts`, also `POST`

Returns the audio itself; X-Voice and X-Voice-Licence headers name the voice and its licence. Up to 3,000 characters; split longer text. Write the text in its own script: Hindi in Devanagari, Telugu in Telugu, Malayalam in Malayalam.

POST body: The text to speak. Send it as `text/plain`, or as JSON `{"text": "..."}` with the other options beside it.

What you send is never written to the log, and the answer is not cached.

Parameters:

- `text` (query): What to say (or POST it).
- `voice` (query): A voice ID from /v1/tts/voices, or a language (hi, te, ml, en, en-GB) for its first voice that may be used commercially. Default en.
- `format` (query): Default mp3. One of: mp3, ogg, wav.
- `speed` (query): 0.5 to 2. Default 1.

Example: https://api.dharun.dev/v1/tts?text=Namaste%2C%20welcome%20to%20India%20API.&voice=en_GB-cori-medium (returns the audio itself)

## Voices

`GET /v1/tts/voices`

Every voice with its language, speaker, licence and the credit the licence asks for.

Example: https://api.dharun.dev/v1/tts/voices

## Remove background noise

`POST /v1/audio/denoise`

Send a recording (MP3, WAV, Ogg, M4A, WebM, video files too) and get it back with fans, traffic and hum lowered, as mono 48 kHz. The first five minutes are cleaned. Options go in the query string, or as form fields when the file is sent as multipart/form-data.

Send: An audio or video file, up to 10 MB. Post the file as the request body, or as `multipart/form-data`. Sample file: https://api.dharun.dev/samples/noisy.mp3

What you send is never written to the log, and the answer is not cached.

Parameters:

- `format` (query): Default mp3. One of: mp3, ogg, wav.
- `limit` (query): The most noise is lowered by, in decibels, 6 to 100. Lower values sound more natural. Default 100.

Example: `POST` https://api.dharun.dev/v1/audio/denoise (returns the audio itself)
