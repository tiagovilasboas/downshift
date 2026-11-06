# OmniRoute: what the official sources confirm (2026-10-20)

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

Research for the OmniRoute adapter behind the router-agnostic gateway port
(`internal/gateway`). Official pages only: the project repository, its README and
its wiki and user guide. Every page was read on **2026-10-20** with WebFetch. No
account was created, no API key was used or requested, and no endpoint was called.
Third-party write-ups and issue threads found by search were not used as evidence.

Status words: **CONFIRMED** (an official page says it), **INFERRED** (follows from
official pages but is not stated), **UNKNOWN** (no official page answers it). An
UNKNOWN fact is not implemented.

Sources:

- S1: https://github.com/diegosouzapw/OmniRoute (README, read 2026-10-20)
- S2: https://github.com/diegosouzapw/OmniRoute/wiki/API-Reference (read 2026-10-20)
- S3: https://github.com/diegosouzapw/OmniRoute/blob/main/docs/guides/USER_GUIDE.md (read 2026-10-20)

## Facts

| # | Fact | Status | Source |
|---|------|--------|--------|
| 1 | OmniRoute is an MIT-licensed, OpenAI-compatible AI gateway that you run yourself. | CONFIRMED | S1 |
| 2 | Default local endpoint is `http://localhost:20128/v1`. | CONFIRMED | S1, S3 |
| 3 | Model ids are `<provider>/<model>`, e.g. `cc/claude-opus-4-6`, `openai/gpt-5.4`, `gh/gpt-5.5`. | CONFIRMED | S2, S3 |
| 4 | The provider prefix is a route name (`cc`, `gh`, `if`), not a vendor name. A mapping from prefix to vendor is not documented. | INFERRED | S3 |
| 5 | Some ids have more than one slash, e.g. `nebius/Qwen/Qwen3-Embedding-8B`. | CONFIRMED | S2 |
| 6 | `auto` is a model value that routes across connected providers. `auto/coding` is listed among example ids. | CONFIRMED | S1, S2 |
| 7 | `GET /v1/models` returns the chat, embedding and image models and the combos, in OpenAI format. | CONFIRMED | S2 |
| 8 | `/v1/*` routes require a Bearer key only when `REQUIRE_API_KEY=true`, and that setting defaults to `false`. Whether `/v1/models` is covered by that switch is not stated. | CONFIRMED (switch); UNKNOWN (coverage of `/v1/models`) | S2, S3 |
| 9 | The list describes the models of the instance that serves it ("connected providers"). It is not a hosted public catalog. | INFERRED | S2, S3 |
| 10 | Per-model price fields are not documented. `GET /api/pricing` exists; its response shape is not described. | CONFIRMED (endpoint exists); UNKNOWN (fields) | S2 |
| 11 | Prices appear in prose as cost per 1K input and output tokens, and in a table as per 1M tokens. Which unit the API uses is not stated. | CONFIRMED (both units in prose); UNKNOWN (API unit) | S3 |
| 12 | Data retention, zero-retention or training use, per upstream provider. Not stated. | UNKNOWN | S1, S2, S3 |
| 13 | OmniRoute records request logs locally, and the instance can purge them (`/api/settings/purge-request-history`). | CONFIRMED | S2, S3 |
| 14 | "Local-first", AES-256-GCM encrypted stored keys, and no upload of prompts for the optional Radar feature. | CONFIRMED | S1 |

## What this decides for the adapter

- **NormalizeID.** Accepts exactly one slash with both parts present (fact 3). Rejects
  `auto` and `auto/...` as routing aliases, not models (fact 6). Rejects
  multi-slash ids, which the port contract also requires (fact 5 is a known gap).
  The prefix is kept as-is; no prefix-to-vendor mapping is implemented (fact 4).
- **Price.** Read only from a snapshot supplied by the composition root. The
  composition root registers the adapter with no snapshot, so OmniRoute prices
  nothing until a source of OmniRoute prices is confirmed (facts 10, 11).
- **Privacy.** Always `unknown` with the source named (fact 12).
- **ListModels.** Returns `gateway.ErrUnsupported`. The list is per instance, and
  whether it needs a key depends on a switch whose coverage is unknown (facts 8, 9).
  A public, keyless list is not confirmed, so nothing is fetched.

## Not implemented (UNKNOWN)

- Per-model prices and their unit (facts 10, 11).
- Data retention per provider (fact 12). Privacy stays `unknown`.
- Model listing (fact 8, 9).
- Prefix-to-vendor mapping (fact 4).
- Multi-slash ids such as `nebius/Qwen/...` (fact 5), which the port rejects.
