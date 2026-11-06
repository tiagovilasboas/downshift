# OpenRouter: the three unverified facts (2026-10-20)

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

Checks the "Not verified yet" list of the OpenRouter plan (`docs/openrouter-plan`
branch, `docs/internal/tasks/2026-10-20-openrouter/README.md`). Official pages only.
Every page below was read on **2026-10-20**. No API key was used and no
authenticated endpoint was called.

Status words: **CONFIRMED** (an official page says it), **INFERRED** (follows from
official pages, or was observed on the live public endpoint, but is not stated),
**UNKNOWN** (no official page answers it).

## Fact 1: model id each harness sends through OpenRouter

The slug `vendor/model` survives end to end in all three harnesses. Two things the
plan did not assume: OpenRouter's own harness guides default to a leading `~`
alias (`~anthropic/claude-opus-latest`), and OpenCode prepends its provider id.

| Harness | What is sent | Status | Source | Read |
|---|---|---|---|---|
| Claude Code, endpoint | `ANTHROPIC_BASE_URL=https://openrouter.ai/api`, key in `ANTHROPIC_AUTH_TOKEN`, `ANTHROPIC_API_KEY` empty | CONFIRMED | https://openrouter.ai/docs/cookbook/coding-agents/claude-code-integration | 2026-10-20 |
| Claude Code, model ids | Per-class overrides `ANTHROPIC_DEFAULT_{FABLE,OPUS,SONNET,HAIKU}_MODEL`; example values are tilde aliases such as `~anthropic/claude-haiku-latest`; pinned form is `anthropic/claude-opus-5`. A `[1m]` suffix is stripped by OpenRouter before routing | CONFIRMED | same page | 2026-10-20 |
| Claude Code, subagents | `CLAUDE_CODE_SUBAGENT_MODEL` sets one model for every spawned subagent. Otherwise a subagent inherits the main model; a subagent's own `model` frontmatter is also honoured | CONFIRMED (env var: OpenRouter page; inherit and frontmatter: Claude Code page) | https://openrouter.ai/docs/cookbook/coding-agents/claude-code-integration ; https://code.claude.com/docs/en/model-config | 2026-10-20 |
| Claude Code, id kept as `vendor/model` | The id string is the OpenRouter slug, sent unchanged (the gateway only changes where requests go, not which model answers) | CONFIRMED for the env var values; INFERRED that the wire `model` field equals that string | same two pages | 2026-10-20 |
| Claude Code, subagent frontmatter with a slug | Frontmatter aliases (`haiku`, `sonnet`, `opus`) resolve through the `ANTHROPIC_DEFAULT_*_MODEL` variables; whether a raw `vendor/model` in frontmatter is passed through is not stated | UNKNOWN | https://code.claude.com/docs/en/model-config (page text was truncated at 100000 of 121250 characters; sub-agents page not read) | 2026-10-20 |
| OpenCode, provider id | `openrouter` is the key under `provider` in `opencode.json` | CONFIRMED | https://opencode.ai/docs/providers/ | 2026-10-20 |
| OpenCode, model id | Full id is `provider_id/model_id`, so `openrouter/` + the slug: `openrouter/~anthropic/claude-sonnet-latest`, or pinned `openrouter/anthropic/claude-sonnet-4.5`. Entries under `provider.openrouter.models` are keyed by the bare slug | CONFIRMED | https://openrouter.ai/docs/cookbook/coding-agents/opencode-integration ; https://opencode.ai/docs/models/ | 2026-10-20 |
| OpenCode, subagent model | `model` key in the agent entry or frontmatter, same `provider/model-id` form; unset means it uses the invoking primary agent's model | CONFIRMED | https://opencode.ai/docs/agents/ | 2026-10-20 |
| Codex, provider | Top-level `model_provider = "openrouter"` plus a `[model_providers.openrouter]` table with `base_url` `https://openrouter.ai/api/v1`; `openai`, `ollama`, `lmstudio` are reserved ids; the keys are only honoured in `~/.codex/config.toml`, not in a project file | CONFIRMED | https://openrouter.ai/docs/cookbook/coding-agents/codex-cli ; https://learn.chatgpt.com/docs/config-file/config-advanced (redirect of developers.openai.com/codex/config-advanced) | 2026-10-20 |
| Codex, model id | Top-level `model` is the full slug, example `~openai/gpt-sol-latest` | CONFIRMED (OpenRouter page); the OpenAI page has no OpenRouter example | https://openrouter.ai/docs/cookbook/coding-agents/codex-cli | 2026-10-20 |
| Codex, auth | OpenRouter's page uses command-backed auth (`[model_providers.openrouter.auth]`), which it says makes Codex load OpenRouter's model catalog; OpenAI's page says not to combine that table with `env_key` | CONFIRMED | same two pages | 2026-10-20 |

Notes that matter for Downshift:

- The `~vendor/name-latest` form is an alias. The models API exposes
  `alias_target` (the concrete slug behind it), and the live public list does carry
  ids that start with `~` (observed, see fact 2).
- Claude Code is not guaranteed to work with non-Anthropic ids: OpenRouter's own
  page says only the Anthropic first-party provider is guaranteed.

## Fact 2: the account-filtered list, ZDR, price units, variant suffixes

| Fact | Status | Source | Read |
|---|---|---|---|
| Account-filtered list is `GET https://openrouter.ai/api/v1/models/user` ("list models filtered by user provider preferences, privacy settings, and guardrails") | CONFIRMED | https://openrouter.ai/docs/api/api-reference/models/list-models-filtered-by-user-provider-preferences-privacy-settings-and-guardrails | 2026-10-20 |
| It needs a bearer key. The 401 example says the header is missing; the 403 example says only management keys can do it, so a plain inference key may be refused | CONFIRMED that a key is needed; UNKNOWN whether a non-management key works | same page | 2026-10-20 |
| Params: `offset`, `limit` (default 500, max 1000), `output_modalities` (default `text`), `sort`. Regional hostnames (`eu.openrouter.ai`) return only models that support in-region routing | CONFIRMED | same page | 2026-10-20 |
| Response: `{data, total_count, links.next}`; `data` holds the same Model objects as the public list | CONFIRMED | same page | 2026-10-20 |
| That response has no ZDR or data-collection field. The Model object lists no such property; privacy settings only filter which models appear | CONFIRMED (absence in the published schema) | same page | 2026-10-20 |
| A model absent from `/models/user` is therefore "not usable under this account's settings", but the reason (ZDR, provider, guardrail) is not reported | INFERRED | same page | 2026-10-20 |
| ZDR is an account setting per model group (Anthropic, OpenAI, Google, others), also set by guardrails; a request can add `provider.zdr: true`, combined with the account setting by OR (it cannot turn account enforcement off) | CONFIRMED | https://openrouter.ai/docs/guides/features/zdr | 2026-10-20 |
| A ZDR endpoint list exists at `https://openrouter.ai/api/v1/endpoints/zdr`; the page shows it fetched with no auth header but does not state the auth rule | CONFIRMED path; UNKNOWN auth. Its response shape was not read: the docs page I tried for it returned 404 | https://openrouter.ai/docs/guides/features/zdr | 2026-10-20 |
| The public list also takes a `zdr=true` filter, plus `providers`, `model_authors`, `region` | CONFIRMED | https://openrouter.ai/docs/api/api-reference/models/get-models | 2026-10-20 |
| The `data_collection` request preference is not documented on the ZDR page | UNKNOWN (not found on pages read) | https://openrouter.ai/docs/guides/features/zdr | 2026-10-20 |
| Public list `pricing` values are strings in USD **per token** (a prompt price of `0.00003` is $30 per million). Filter params `min_price` and `max_price` are the exception: dollars per **million** tokens. A `"0"` price means free | CONFIRMED | https://openrouter.ai/docs/api/api-reference/models/get-models ; https://openrouter.ai/docs/guides/overview/models | 2026-10-20 |
| Conditional prices sit in a `pricing.overrides` array; top-level prices are the defaults | CONFIRMED | same pages | 2026-10-20 |
| Variant suffixes are added after a colon. **Catalog variants** have their own list entries: `:free`, `:batch` (active), `:thinking`, `:extended` (deprecated). **Routing variants** are not list entries and take the base model's metadata: `:nitro`, `:floor`, `:exacto` (active), `:online` (deprecated). Suffixes stack, for example `poolside/laguna-s-2.1:free:nitro`; at most one catalog variant. A catalog suffix on a model without that entry returns 404 or an empty endpoint list | CONFIRMED | https://openrouter.ai/docs/guides/routing/model-variants ; https://openrouter.ai/docs/guides/overview/models | 2026-10-20 |
| Observed on the live public list: 468 models; `:free` and `:batch` ids appear as separate entries; ids starting with `~` appear. Observation of one unauthenticated GET, not documentation | INFERRED | https://openrouter.ai/api/v1/models | 2026-10-20 |

## Fact 3: reading the public list once a day

| Fact | Status | Source | Read |
|---|---|---|---|
| The public list needs no key. The models overview shows plain `curl` calls without an auth header but does not say a key is optional; the API reference spec applies a bearer scheme globally. One unauthenticated GET returned 200 and a body, with `cache-control: public, max-age=120` and a Cloudflare cache hit | INFERRED (observed; docs silent) | https://openrouter.ai/docs/guides/overview/models ; https://openrouter.ai/docs/api/api-reference/models/get-models ; response of https://openrouter.ai/api/v1/models | 2026-10-20 |
| The rate limits page gives numbers only for `:free` models (per minute and per day, depending on credits bought) and says nothing about unauthenticated calls or the models endpoint. It says Cloudflare blocks traffic far above reasonable use, without a threshold | CONFIRMED (limits page is silent on the list endpoint) | https://openrouter.ai/docs/api/reference/limits | 2026-10-20 |
| The FAQ does not say whether the list may be cached or how often it changes | UNKNOWN | https://openrouter.ai/docs/faq | 2026-10-20 |
| Terms of Service (last updated 2026-08-31, section 7) prohibit bots, crawlers or scripts that scrape or copy information from the site or service, and bypassing anti-scraping measures. They do not mention the public model list or a permitted polling rate | CONFIRMED text; whether one documented API call per day counts as scraping is UNKNOWN | https://openrouter.ai/terms | 2026-10-20 |
| Nothing official found that grants or forbids a once-a-day read of the public list from a user machine | UNKNOWN | all pages above | 2026-10-20 |

Reading: the documented API is a published, cacheable, CORS-open JSON endpoint,
and one request per day is far below the cache lifetime the server itself
advertises. That is a reasonable-use argument, not a stated permission.

## What this changes in the plan

Part 3 (discovery source `openrouter`):

- **The account-filtered list does not report ZDR.** Design item 1 and item 6
  assumed it might. It only hides models the account settings exclude, and it
  needs a key (possibly a management key). Keep `zdr` as a user-declared allowlist
  field, as designed; do not infer it from presence or absence in `/models/user`.
  The unauthenticated `zdr=true` filter on the public list, or `/endpoints/zdr`,
  could cross-check a declared flag, but its auth and shape need a read of the
  endpoint reference first. Treat as an open item, not a dependency.
- **Ship only the public list, with no key, in part 3.** The optional
  account-filtered read stays out of part 3 until a key type is confirmed.
- **Ids are not plain `vendor/model`.** The fixtures and the resolver must handle
  `~vendor/name-latest` aliases (use `alias_target`), `:free` and `:batch`
  entries, and routing suffixes (`:nitro`, `:floor`, `:exacto`) that are never in
  the list. Today `normaliseModelID` in `internal/catalog/catalog.go` cuts at the
  first `/` only, so `~anthropic/claude-haiku-latest` and `...:free` would not map to
  catalog entries cleanly. Part 1 needs tests for these forms.
- **Price units.** Per token as strings, not per million. The existing price sync
  and any new column must convert once, in one place.
- **Cadence risk.** The terms prohibit scripted scraping of the service and say
  nothing about the public list. Keep the read opt-in, no more than once a day,
  with a cache honouring the advertised max-age, a clear `User-Agent`, and
  failing open to the embedded catalog. Ask the owner whether to email OpenRouter
  for an explicit statement before enabling it by default.

Part 5 (harness wiring facts):

- **Claude Code.** Sub-agent models come from `CLAUDE_CODE_SUBAGENT_MODEL` or the
  subagent frontmatter, while the main classes come from `ANTHROPIC_DEFAULT_*_MODEL`.
  Whether Downshift's `updatedInput.model` rewrite of a slug reaches the gateway
  unchanged is INFERRED only; the frontmatter-with-slug case is UNKNOWN. Verify
  both in a live session before part 6. The default OpenRouter setup uses
  `-latest` aliases, so Downshift will often see an alias, not a pinned slug.
- **OpenCode.** The id is `openrouter/<slug>`. Plan item "OpenCode carries a
  `providerID` and `id`" matches: `providerID=openrouter`, id is the slug, which
  can itself contain a `/` and a leading `~`. The resolver must not split the
  OpenCode id at the first `/` twice.
- **Codex.** Provider config lives only in `~/.codex/config.toml`, never in a
  project file, and OpenRouter's page uses command-backed auth. Downshift must
  not edit that file without the usual dry-run and consent. The model is the full
  slug in top-level `model`.
- **Contradiction with the plan:** the plan says same-vendor moves "map onto
  existing entries and tiers". That holds for pinned `anthropic/...` slugs, but
  the documented default is `~anthropic/claude-<class>-latest`, which the current
  prefix strip does not resolve to a pinned version. Part 5 should decide whether
  an alias maps to its class tier or to `alias_target`.

Still open after this pass: `/models/user` with a non-management key,
`/endpoints/zdr` auth and shape, `data_collection` as a request preference, and
Claude Code frontmatter with a raw slug.
