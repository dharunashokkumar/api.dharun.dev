# Image tools API

> Resize and convert pictures, remove backgrounds, and compare pictures by how they look.

Upload a picture and get it back resized or in another format (WebP, AVIF, JPEG, PNG), with its background removed, or as a fingerprint for finding duplicates. HEIC photos from iPhones are read too. Metadata such as the GPS position is always removed. Pictures are processed and deleted; nothing is kept or logged.

Base URL `https://api.dharun.dev`. No API key. Answers are JSON, `{ "data": ..., "meta": { ... } }`, and errors are `{ "error": { "code", "message" } }` with a matching HTTP status. Rate limit: 600 requests a minute per client.

## Resize and convert a picture

`POST /v1/image/convert`

Returns the picture itself, turned upright from its EXIF orientation, without metadata. fit=inside keeps the whole picture within width and height and never enlarges it; cover fills the box and crops the middle; fill stretches. Options go in the query string, or as form fields when the file is sent as multipart/form-data.

Send: A picture: JPEG, PNG, WebP, HEIC, AVIF, GIF, BMP or TIFF, up to 10 MB. Post the file as the request body, or as `multipart/form-data`. Sample file: https://api.dharun.dev/samples/photo.jpg

What you send is never written to the log, and the answer is not cached.

Parameters:

- `format` (query): Output format. Default webp. One of: jpeg, png, webp, avif, gif, tiff.
- `width` (query): Width in pixels, 1 to 8000.
- `height` (query): Height in pixels, 1 to 8000.
- `fit` (query): How the picture fits width and height. Default inside. One of: inside, cover, fill.
- `quality` (query): JPEG, WebP and AVIF quality, 1 to 100. Default 80.

Example: `POST` https://api.dharun.dev/v1/image/convert?format=webp&width=400 (returns the image itself)

## Size, format and metadata of a picture

`POST /v1/image/info`

Format, size, colour space, pages, orientation, the camera, and whether the picture carries EXIF data or a GPS position.

Send: A picture: JPEG, PNG, WebP, HEIC, AVIF, GIF, BMP or TIFF, up to 10 MB. Post the file as the request body, or as `multipart/form-data`. Sample file: https://api.dharun.dev/samples/photo.jpg

What you send is never written to the log, and the answer is not cached.

Example: `POST` https://api.dharun.dev/v1/image/info

## Remove the background

`POST /v1/image/remove-background`

Returns the picture as a PNG with the background transparent, cut out by the IS-Net model. Works best on one clear subject: a product, a person, an animal. Takes a few seconds; pictures larger than 4096 pixels are made smaller. Options go in the query string, or as form fields when the file is sent as multipart/form-data.

Send: A picture: JPEG, PNG, WebP, HEIC, AVIF, GIF, BMP or TIFF, up to 10 MB. Post the file as the request body, or as `multipart/form-data`. Sample file: https://api.dharun.dev/samples/photo.jpg

What you send is never written to the log, and the answer is not cached.

Parameters:

- `format` (query): png (default), webp, or mask: the cut-out mask alone, white where the subject is. One of: png, webp, mask.

Example: `POST` https://api.dharun.dev/v1/image/remove-background (returns the image itself)

## Perceptual hash of a picture

`POST /v1/image/hash`

Three 64-bit fingerprints (average, difference and DCT-based perceptual hashes) that stay nearly the same when a picture is resized, recompressed or slightly edited. Pictures whose perceptual hashes differ by 10 bits or fewer look alike.

Send: A picture: JPEG, PNG, WebP, HEIC, AVIF, GIF, BMP or TIFF, up to 10 MB. Post the file as the request body, or as `multipart/form-data`. Sample file: https://api.dharun.dev/samples/photo.jpg

What you send is never written to the log, and the answer is not cached.

Example: `POST` https://api.dharun.dev/v1/image/hash

## Compare two pictures

`POST /v1/image/compare`

Send two pictures as form fields. Returns the bits by which each hash differs, a score from 0 to 1, and whether they look like the same picture.

Send: Two pictures, as multipart/form-data, up to 10 MB each. Post the file as the request body, or as `multipart/form-data` with one field per file. Sample file: https://api.dharun.dev/samples/photo.jpg

What you send is never written to the log, and the answer is not cached.

Example: `POST` https://api.dharun.dev/v1/image/compare
