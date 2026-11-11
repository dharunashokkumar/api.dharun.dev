# Hospitals API

> About 30,000 hospitals across India with their kind, system of medicine, phone and position, from the National Hospital Directory.

The Ministry of Health's National Hospital Directory: government and private hospitals, nursing homes and clinics, allopathic and AYUSH, with address, phone numbers, emergency and ambulance numbers where given, specialties, and a position for about a third. As of May 2016: check before you rely on one. In an emergency dial 112 or 108.

Base URL `https://api.dharun.dev`. No API key needed for most endpoints; a free key (https://api.dharun.dev/keys) raises the limits, and the model endpoints marked below will need one. Answers are JSON, `{ "data": ..., "meta": { ... } }`, and errors are `{ "error": { "code", "message" } }` with a matching HTTP status. Rate limit: 600 units a minute per address without a key, 3000 per key; the `RateLimit-*` headers say what is left.

## Hospitals near a point, by PIN code, district or name

`GET /v1/hospitals`

One of near, pincode, district or q. near gives the ten nearest within about 50 km, with their distance; the others up to 100, by name.

Parameters:

- `near` (query): latitude,longitude.
- `pincode` (query): Six digits.
- `district` (query): A district, such as Namakkal; add state to narrow it.
- `state` (query): With district or q.
- `q` (query): Part of a hospital's name, 3 to 80 characters.
- `system` (query): Only this system of medicine: Allopathic, Ayurveda, Homeopathy, Siddha, Unani.

Example: https://api.dharun.dev/v1/hospitals?pincode=637001&utm_medium=skill
