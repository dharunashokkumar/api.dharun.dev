# IPC to BNS section finder API

> Which section of the Bharatiya Nyaya Sanhita replaced IPC 302, and the same for CrPC to BNSS and the Evidence Act to BSA.

India's new criminal laws came into force on 1 July 2024: the Bharatiya Nyaya Sanhita (BNS) replaced the Indian Penal Code (IPC), the Bharatiya Nagarik Suraksha Sanhita (BNSS) the Code of Criminal Procedure (CrPC), and the Bharatiya Sakshya Adhiniyam (BSA) the Indian Evidence Act (IEA). Look a section up either way, or search the titles. From the Bureau of Police Research and Development's correspondence tables; the section titles are the Acts' own.

Base URL `https://api.dharun.dev`. No API key needed for most endpoints; a free key (https://api.dharun.dev/keys) raises the limits, and the model endpoints marked below will need one. Answers are JSON, `{ "data": ..., "meta": { ... } }`, and errors are `{ "error": { "code", "message" } }` with a matching HTTP status. Rate limit: 600 units a minute per address without a key, 3000 per key; the `RateLimit-*` headers say what is left.

## A section and its counterpart

`GET /v1/law/{law}/{section}`

With a new law (bns, bnss, bsa), the section (all its subsections for 103; one for 103(1)) and the old section it stands for. With an old law (ipc, crpc, iea), every new section the old one went to. new_provision marks a section with no counterpart.

Parameters:

- `law` (path, required): Which law the section number belongs to. One of: bns, bnss, bsa, ipc, crpc, iea.
- `section` (path, required): Such as 302, 498A, 103(1).

Example: https://api.dharun.dev/v1/law/ipc/302?utm_medium=skill

## Search section titles

`GET /v1/law/{law}`

The new law's sections whose titles contain the words (an old law's name searches its replacement), at most 50.

Parameters:

- `law` (path, required): Which law. One of: bns, bnss, bsa, ipc, crpc, iea.
- `q` (query, required): 3 to 100 characters, such as cheating or bail.

Example: https://api.dharun.dev/v1/law/bns?q=murder&utm_medium=skill
