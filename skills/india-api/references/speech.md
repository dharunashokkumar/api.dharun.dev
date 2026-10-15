# Speech API

> Speech to text, text to speech in Hindi, Telugu, Malayalam and English, and noise removal.

Turn a recording into text (and subtitles) with Whisper, turn text into natural speech with Piper voices, or clean background noise from a recording with DeepFilterNet. Each voice keeps the licence of the recordings it learnt from: /v1/tts/voices lists them, and two Hindi voices are for non-commercial use only. There is no Tamil, Kannada, Bengali, Gujarati or Marathi voice yet. Nothing you send is kept or logged.

Base URL `https://api.dharun.dev`. No API key. Answers are JSON, `{ "data": ..., "meta": { ... } }`, and errors are `{ "error": { "code", "message" } }` with a matching HTTP status. Rate limit: 600 requests a minute per client.

## Speech to text, with subtitles

`POST /v1/stt`

Send a recording (MP3, WAV, Ogg, M4A, WebM, video files too) and get the text with timed segments, as JSON, SRT or WebVTT subtitles, or plain text. Whisper small recognises about a hundred languages, Indian ones included; it is good on clear English and fair on Indian languages. task=translate gives English text whatever was spoken. The first five minutes are heard. Options go in the query string, or as form fields when the file is sent as multipart/form-data.

Send: An audio or video file, up to 10 MB. Post the file as the request body, or as `multipart/form-data`. Sample file: https://api.dharun.dev/samples/speech.mp3

What you send is never written to the log, and the answer is not cached.

Parameters:

- `language` (query): auto (default) or the language spoken: en, hi, ta, te, kn, ml, mr, bn, gu, pa, ur, as, ne, sa, sd, or, ar, fr, de, es, zh, ja. Naming it helps on short clips.
- `task` (query): transcribe (default) writes what was said; translate writes it in English. One of: transcribe, translate.
- `format` (query): Default json. One of: json, srt, vtt, text.

Example: `POST` https://api.dharun.dev/v1/stt

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
