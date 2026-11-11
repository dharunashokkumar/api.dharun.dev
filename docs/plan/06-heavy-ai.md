# Parts 7 to 9: document AI, a small hosted LLM, the code sandbox

Dharun, 2026-11-10: all three are wanted. They are offered only with a free API key
([03-accounts-and-keys.md](03-accounts-and-keys.md)), so every one of them depends on
Part 2.

Model names, licences and speeds below are from memory and **must be checked and
measured** before anything is chosen. The repository's rule stands: Go and ONNX or a
small native helper on the server, no Python; models are server-only datasets pinned by
checksum.

## Part 7: document AI without an LLM

Needs no new infrastructure: OCR, poppler, the validators and the name models are on
the server already. Every endpoint is an upload, Private, key only.

| ★ | Idea | Address | How |
|---|---|---|---|
| ★ | **Document to Markdown** | `POST /v1/doc/markdown` | A PDF's text layer with positions (poppler), or OCR with positions for scans; headings from font size, lists, tables, reading order across columns. What agents and RAG pipelines want from a PDF |
| ★ | **Table extraction** | `POST /v1/doc/tables` | Ruled and whitespace-aligned tables to CSV or JSON |
| ★ | **Invoice reader** | `POST /v1/doc/invoice` | GSTINs (checksum verified), invoice number and date, HSN codes (looked up), taxable value, CGST, SGST, IGST and total, with a check that the sums agree. The reverse of `/v1/invoice` |
| ★ | **Aadhaar masking** | `POST /v1/doc/mask-aadhaar` | Finds the 12-digit number in an image (Verhoeff checked) and returns the image with the first eight digits covered, as the masked-Aadhaar rule asks. A real compliance chore for anyone who collects ID copies. The number is never returned or logged |
| ★ | **E-invoice QR reader** | `POST /v1/doc/einvoice-qr` | The QR on a GST e-invoice is a signed token. Decode it, show seller, buyer, invoice number, value and IRN, and verify the signature against the portal's published keys. Says whether the invoice is what it claims |
| | Receipt reader | `POST /v1/doc/receipt` | Merchant, date, total, tax from a photo of a bill |
| | Cheque reader | `POST /v1/doc/cheque` | The MICR line and IFSC from a cheque photo, joined to the bank and branch |
| | Bank statement to rows | `POST /v1/doc/statement` | Statement PDF to dated transactions. Layouts differ by bank; start with the text-layer PDFs of the largest banks. High demand, the most work |
| | Redact a document | `POST /v1/doc/redact` | The text redaction that exists, applied to a PDF or image: personal details covered in the returned file |

**Status (2026-11-11):** all nine endpoints are built on branch `growth-catalogue`. The e-invoice QR is decoded but its signature is not checked: dropped (Dharun, 2026-11-11), as no published key could be found.

Limits to state on the pages: these read what is printed; they do not prove a document
is genuine (the e-invoice signature check is the exception).

## Part 8: a small hosted LLM

**What the machine can do.** CPU only, 10 cores. A model of about 1 billion parameters
at 4-bit should give tens of tokens a second, a 3 to 4 billion one perhaps 10 to 20;
measure on the server before promising anything. Memory 1 to 3 GB while loaded, inside
the service's current limit if it replaces the idle text helpers rather than adding to
them.

**How.** `llama.cpp` (MIT) as a helper process, started on first use and stopped when
idle, the way whisper.cpp and the ONNX helpers are; built in the release. One
generation at a time through the model slot, with a queue.

**Shape, two layers:**

| Layer | Address | Why |
|---|---|---|
| Task endpoints | `/v1/llm/summarise`, `/classify`, `/extract` (JSON that must match a schema the caller sends, enforced by constrained decoding), `/rewrite` | Small models are reliable when the task is narrow and the output is forced into shape. These are the honest product |
| Chat, OpenAI-compatible | `/v1/chat/completions` | Students can point existing code and tools at it by changing the base URL. The draw is large and so is the abuse; small context, small output cap, daily token quota per key |

**Model candidates** (licences to confirm): the small Qwen models (Apache-2.0), SmolLM
(Apache-2.0), Phi mini (MIT). Gemma's terms are not an open licence. For Indian
languages, test each candidate on Hindi and Tamil before choosing: a model that is
fluent only in English is half a product here. A fine-tune with clear terms for Indian
languages would be worth more than a larger general model.

**Rules:** key only; a daily token quota per key (proposed: 100,000); prompts and
answers never logged (Private); moderation is the caller's job, and the page says the
model can be wrong; the queue length is on the dashboard, and the endpoint answers 503
with `Retry-After` when the wait is too long.

**Decisions for Dharun:** which model after the measurements; whether chat is offered
or only the task endpoints; the quota.

## Part 9: the code sandbox

The design is in `docs/sandbox-design.md` (2026-10-14) and the VM was requested from the
homelab on 2026-10-17. What changes with this plan:

- **Login:** the sandbox design assumed GitHub login and Turnstile. It now uses the
  accounts and keys from Part 2: a key, a daily quota of runs and CPU seconds per key,
  and the audit log keyed by account.
- **"No API keys" and "GitHub only"** are no longer conflicts; "one container" still
  is (the sandbox needs its own VM), and that exception was already accepted in the
  design.
- The seven decisions at the end of the design document are still Dharun's to make.
  Read it again against decisions 8 to 10 of this plan and update it before building.

Order inside the part stays as designed: run-once (code in, output out, no network,
30 seconds), then kept-alive sessions with files, then anything interactive.

What it unlocks elsewhere: a "run this example" button in the docs and the playground
(Python and JavaScript samples executed for real), and safe execution for the request
bin's test scripts.

## Monitoring for all three

Already decided on 2026-10-14 for services like these: per-job resources, abuse
signals, an audit log that is never public, and a kill switch whose state shows on the
dashboard. With accounts, add per-key use of each heavy service and the top keys by
use, visible only on the private dashboard.

## Open questions

1. Which document AI endpoints first (recommended: Markdown, tables, invoice reader,
   Aadhaar masking, e-invoice QR).
2. LLM: chat or task endpoints only; the model; the quota.
3. Sandbox: the seven decisions in `docs/sandbox-design.md`, re-read with keys in place.
4. Quotas per key for each heavy service.
