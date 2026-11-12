# Census 2011 figures API

> Population, households, literacy, sex ratio and workers for India, every state, district, sub-district and town, from the 2011 census.

The Primary Census Abstract of the Census of India 2011, the latest count published: households, people by sex, children under seven, Scheduled Castes and Tribes, literacy and work, each as a total and split into rural and urban, with the literacy rate, sex ratio and other rates worked out. 35 states and union territories, 640 districts, 5,923 sub-districts and 7,933 towns, as they were in 2011.

Base URL `https://api.dharun.dev`. No API key needed for most endpoints; a free key (https://api.dharun.dev/keys) raises the limits, and the model endpoints marked below will need one. Answers are JSON, `{ "data": ..., "meta": { ... } }`, and errors are `{ "error": { "code", "message" } }` with a matching HTTP status. Rate limit: 600 units a minute per address without a key, 3000 per key; the `RateLimit-*` headers say what is left.

## Figures for a place, and the places inside it

`GET /v1/census/{level}/{code}`

figures holds total, rural and urban (a town has urban only); rates the literacy rate (of people aged 7 and over), sex ratio (females per 1,000 males), child sex ratio, work participation and the urban share. children lists the places one level down with their population. India is /v1/census/india/0.

Parameters:

- `level` (path, required): Which kind of place. One of: india, state, district, subdistrict, town.
- `code` (path, required): The 2011 census code: 33 for Tamil Nadu, 632 for Coimbatore district (from a search).

Example: https://api.dharun.dev/v1/census/district/632?utm_medium=skill

## Find a place by name

`GET /v1/census`

Places whose names contain the words, exact names first, then the largest; at most 50, each with its code and population.

Parameters:

- `q` (query, required): 3 to 60 letters, such as madurai.
- `level` (query): Only this kind of place. One of: state, district, subdistrict, town.

Example: https://api.dharun.dev/v1/census?q=coimbatore&utm_medium=skill
