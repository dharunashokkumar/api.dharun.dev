# Villages and LGD codes API

> Every state, district, sub-district and village of India with its LGD and Census 2011 codes, and village PIN codes.

The Local Government Directory (LGD) of the Ministry of Panchayati Raj gives every administrative unit a code that government systems use. Walk down from states to districts, sub-districts and villages, find a village by name or by PIN code, or look one up by its code. From data.gov.in, refreshed monthly; names in English and the local script.

Base URL `https://api.dharun.dev`. No API key needed for most endpoints; a free key (https://api.dharun.dev/keys) raises the limits, and the model endpoints marked below will need one. Answers are JSON, `{ "data": ..., "meta": { ... } }`, and errors are `{ "error": { "code", "message" } }` with a matching HTTP status. Rate limit: 600 units a minute per address without a key, 3000 per key; the `RateLimit-*` headers say what is left.

## States with their LGD codes

`GET /v1/lgd/states`

Example: https://api.dharun.dev/v1/lgd/states?utm_medium=skill

## Districts of a state

`GET /v1/lgd/districts`

Parameters:

- `state` (query, required): The state's LGD code, such as 33 for Tamil Nadu.

Example: https://api.dharun.dev/v1/lgd/districts?state=33&utm_medium=skill

## Sub-districts (taluks, tehsils, mandals) of a district

`GET /v1/lgd/subdistricts`

Parameters:

- `district` (query, required): The district's LGD code.

Example: https://api.dharun.dev/v1/lgd/subdistricts?district=580&utm_medium=skill

## Villages by sub-district, PIN code or name

`GET /v1/lgd/villages`

One of subdistrict, pincode or q. Each village with its sub-district, district and state.

Parameters:

- `subdistrict` (query): A sub-district's LGD code: all its villages.
- `pincode` (query): Six digits: the villages LGD lists under it.
- `q` (query): The start of a village's name, 3 to 80 characters; at most 100 answers.
- `state` (query): With q: only this state (LGD code).

Example: https://api.dharun.dev/v1/lgd/villages?pincode=637001&utm_medium=skill

## A village by its LGD code

`GET /v1/lgd/villages/{code}`

Parameters:

- `code` (path, required): The village's LGD code.

Example: https://api.dharun.dev/v1/lgd/villages/634584?utm_medium=skill
