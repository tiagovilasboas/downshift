<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# Value per harness and per API: what is measured, what blocks, what the catalog prices

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

Checked against the repository on 2026-10-19: `internal/core/capabilities.go` (`HarnessCapabilities`, `UsageSource`),
`internal/catalog/catalog.json` (`updated` and `prices_as_of` 2026-10-19), [harness-matrix.md](harness-matrix.md) and
[evidence.md](../guides/evidence.md). This page answers one question per harness: how much of "what did routing do for
me here" is observed, and how much is arithmetic on list prices. It changes no code and no catalog value.

## What a routed spawn changes, and what it does not

Routing does not reduce tokens. The child does the same work and reads and writes about the same tokens, on a
different model. The only thing that changes is the price per token. The dollar saving Downshift reports is therefore
the **same tokens priced on the requested model and again on the model that ran**
([evidence.md](../guides/evidence.md#b-saving-estimated-by-telemetry)). The tokens are an observation (where a harness
reports them). The dollars are an estimate. The baseline (the tokens on the model nobody ran) is a counterfactual.

Two consequences:

- Where a plan has no per-token bill (Claude Code Pro, Max and Team; Cursor plans; a Codex or Antigravity plan quota),
  the invoice does not move with the model in the way list prices suggest. **Tokens by tier** (how many observed tokens
  ran on the small, mid and frontier model) is the more faithful figure there, because it is what was observed. Dollars
  stay a list-price estimate in every case.
- A catalog price is not an invoice ([evidence.md](../guides/evidence.md#why-catalog-prices-are-not-the-invoice)).

## Measurement matrix

"Today" means a record exists in the event log when the harness behaves as in the cited evidence. Source is where
Downshift reads the fact: `payload` (the hook input), `transcript` (a file the payload points to), `none` (nothing is
read). Status words are those of [harness-matrix.md](harness-matrix.md#status-per-harness).

| Harness | Status | Decisions | Rewrite emitted | Rewrite honored | Child end | Child tokens | Dollars |
|---|---|---|---|---|---|---|---|
| Claude Code | Completed | yes, payload | yes, `updatedInput.model` | **observed** 2026-10-06 (child transcript carries the API-reported model) | yes, `SubagentStop`, source transcript | **yes**, `UsageSource: transcript` (`agent_transcript_path`) | estimated: tokens x catalog list price, linked to the decision by `tool_use_hash` |
| Cursor | Functional | yes, payload | yes, `updated_input.model` | **unconfirmed** (silently discarded on Free and legacy Pro; unconfirmed on usage-based Pro and Ultra) | `subagentStop`, **unverified**; links only when the payload repeats the tool-call id | **none** (`UsageSource: none`) | none per spawn |
| Codex | Functional | yes, payload (`model` on every payload) | yes, `updatedInput.model` and `reasoning_effort` | **inferred** 2026-10-02 (a later spawn asked for the model the hook wrote) | `PostToolUse` on `spawn_agent`, **unverified** | **none** (`UsageSource: none`); a rollout `token_count` parser exists but stays off | none per spawn |
| OpenCode | Beta | yes, plugin envelope | yes, route answer with target model | **not observed**, no live spawn yet | `child_done` envelope, **unverified**, no live run | **none** (`UsageSource: none`); the parser reads inline counts but nothing is priced | none: every catalog entry is unpriced |
| Antigravity | Experimental | yes, payload | yes, tier alias (`flash_lite`, `flash`, `pro`) | one exact child observed 2026-10-09 (`pro` requested, `flash_lite` executed) | none declared | none | none per spawn |
| KiroCrew | Experimental | yes, policy mode | no in-place rewrite; block and retry observed once 2026-10-10 | not applicable | none declared | none | none per spawn |

**Only Claude Code records child tokens today.** Every dollar figure that `downshift stats` or `downshift report`
prints per spawn (`actual_cost_usd`, `real_saved_usd`, `real_overspend_usd`) comes from Claude Code transcripts. For the
other five harnesses Downshift records the decision, and for some a rewrite or an end status, never the tokens that ran.
Any "tokens rerouted" figure below is a Claude Code figure until a second harness gets a proven token source.

### What blocks per-token measurement, per harness

| Harness | Blocker | Needed to unblock | Evidence |
|---|---|---|---|
| Cursor | No usage field is read at `subagentStop`; the child executor model is absent from the transcript; Cursor sends no session id; the native quota export is blocked on Cursor 3.24.9 (and is a quota source, not a token one) | A captured `subagentStop` payload that carries child tokens or a child transcript path | [cursor-gap-diagnostic.md](../evidence/cursor-gap-diagnostic.md), [cursor-rewrite-honored-2026-10-10.md](../evidence/cursor-rewrite-honored-2026-10-10.md) |
| Codex | `transcript_path` is the parent rollout until a real payload shows a child-scoped one, so a `token_count` read could return the parent thread's tokens and be booked to the child | One real `PostToolUse` payload for `spawn_agent` showing a child-scoped rollout | `internal/core/capabilities.go` (CodexCaps comment), [harness-matrix.md](harness-matrix.md#child-end-n5-2026-10-13) |
| OpenCode | No token seam is proven (the bus fallback is "NOT PROVEN" in the adapter); no live spawn; every catalog entry is unpriced | A live plugin run that forwards `child_done` with counts, and a list price for the model (or an accepted "unpriced" report) | [harness-matrix.md](harness-matrix.md), `internal/core/capabilities.go` (OpenCodeCaps comment) |
| Antigravity | No documented subagent-end payload; PostToolUse result fields are undocumented; quota mapping unresolved | A vendor-documented end payload | [antigravity-executor-ack.md](../evidence/antigravity-executor-ack.md), [vendor-hook-docs-2026-10-11.md](../evidence/vendor-hook-docs-2026-10-11.md) |
| KiroCrew | PostToolUse `tool_response` not confirmed; no supported non-interactive credit source | A confirmed end payload; a rewrite channel does not exist, so honored cannot apply | [kirocrew-native-hook-2026-10-10.md](../evidence/kirocrew-native-hook-2026-10-10.md) |
| Claude Code | Parent session tokens are outside every hook (only subagents are seen); `PostToolUseFailure` payload never captured, so failed children undercount | Not blocking for child tokens; the parent's own cost needs the harness's own report | [billing-gap-audit.md](../evidence/billing-gap-audit.md) |

### Which catalog provider and price class applies

All prices are USD per 1M tokens from `internal/catalog/catalog.json` (list prices, dated). The automatic target of a
tier is the first non-`explicit_only` entry of that harness and tier in file order (`Catalog` index build in
`internal/catalog/catalog.go`), or the first session model of that tier when a session list is in force
(`sessionResolver.ModelFor`, `internal/core/session_resolver.go`).

| Harness | Provider and API | Automatic small / mid / frontier (input/output) | Cache prices | Billing shape the figure is compared with |
|---|---|---|---|---|
| Claude Code | Anthropic API (also `grok-4.6`, xAI, never automatic) | `claude-haiku-4-5` 1/5, `claude-sonnet-5-5` 2/10, `claude-opus-5-5` 4/20 | read and write on every Anthropic row | Console API per token; Pro, Max and Team have no per-token line |
| Cursor | Mixed: Anthropic, Cursor, Google, OpenAI, xAI, Zhipu | `claude-4.5-haiku-thinking` 1/5, `claude-4.5-sonnet-thinking` 3/15, `claude-opus-5-thinking-high` 5/25 | **none on any Cursor entry** | Cursor plan pools; catalog notes say the costs "rank models inside a session" and "are not a provider invoice" |
| Codex | OpenAI | `gpt-6-luna` 0.1/0.5, `gpt-5.6-terra` 2/12, `gpt-6-sol` 2/10 | read and write on every Codex row | OpenAI API per token; ChatGPT plans meter by quota, never per model |
| OpenCode | OpenCode zen | seven entries, **all unpriced** (`big-pickle` is the frontier entry) | none | no published list price; the catalog says zero fields mean "not in the catalog, never a real $0" |
| Antigravity | Google Gemini via Antigravity aliases | `flash_lite` 0.075/0.3, `flash` 0.075/0.3, `pro` 1.5/6 | none | Google account quota per window, not tied to an alias |
| KiroCrew | Anthropic via Kiro | `claude-haiku-4-5` 1/5, `claude-sonnet-5-5` 2/10, `claude-opus-5-5` 4/20 | read and write | Kiro credits with a `rate_multiplier` per model, so list price does not map to credits |

## Catalog audit: accounting anomalies for "value per API"

Method: read every priced entry of `internal/catalog/catalog.json`, order the tiers per harness, and check whether a
`Downshift` verdict can move to a pricier model. Whether it can does not depend on a guardrail: `compareToCurrentModel`
(`internal/core/policy.go:310-328`) decides a downshift from tier rank alone, and `Savings` (`policy.go:39`) is
informational (`SavingsRatio` returns 0 when the target is not cheaper, `internal/catalog/catalog.go:487-494`). No
path in `internal/core` gates a cross-tier downshift on price before an adapter writes the rewrite (price
comparisons exist only to order equal-tier native candidates, `internal/core/session_native.go:122-131`), so a tier
inversion in the data is an inversion in the money. `RealSavingsUSD` clamps each event at zero
(`internal/telemetry/cost.go:58-70`), so the overspend would show only in `real_overspend_usd`, not in `real_saved_usd`.

| # | Anomaly | Where | Can a routed downshift overspend? |
|---|---|---|---|
| 1 | **Codex frontier is priced below mid.** `gpt-6-sol` (frontier, the automatic frontier target) is 2/10 with cache read 0.20; `gpt-5.6-terra` (mid, the only mid entry) is 2/12 with cache read 0.20. Same input, cache and write prices, output 20% higher on the "cheaper" tier. `gpt-6.1-sol` (frontier) is 2/10 with cache read **0.10**, so terra costs more on cache reads too | `internal/catalog/catalog.json:372-385` (`gpt-6-sol`), `:386-399` (`gpt-6.1-sol`), `:414-427` (`gpt-5.6-terra`) | **Yes.** A session on `gpt-6-sol` or `gpt-6.1-sol` routed to mid (`gpt-5.6-terra`) is a Downshift verdict that costs about 20% more on output and, from `gpt-6.1-sol`, 2x on cached reads. `gpt-5.6-sol` (4/20) to terra (2/12) does save. Also the reverse: a mid-to-frontier upshift from terra to `gpt-6-sol` is cheaper. Whether `gpt-6-sol` is truly cheaper than `gpt-5.6-terra` at the vendor is not verified here; the codex rows were last checked on 2026-10-19 against the OpenAI pricing page per `_doc8` |
| 2 | **Cursor frontier below mid.** `grok-4.7-xhigh` (frontier) is 2/6; the automatic mid `claude-4.5-sonnet-thinking` is 3/15. The other Cursor mids (`cursor-grok-4.6-medium` 2/6 at `:259-271`, `glm-5.2-high` 2/8 at `:272-284`, `claude-sonnet-5-5-high` 2/10 at `:246-258`) sit below the 4/20 and 5/25 frontier rows, so the inversion is specific to the Grok frontier row | `catalog.json:194-205` (`grok-4.7-xhigh`), `:166-177` (`claude-4.5-sonnet-thinking`) | **Yes.** A Cursor session on `grok-4.7-xhigh` routed to the automatic mid is a Downshift verdict at 3/15 against 2/6, 2.5x the output price. Needs a session whose current model is Grok |
| 3 | **Claude Code frontier `grok-4.6` below mid.** `grok-4.6` (xAI, frontier tier) is 2/6 with cache read 0.50 and no write price; the automatic mid `claude-sonnet-5-5` is 2/10 with cache read 0.10. Mixed: 67% more on output, 5x cheaper on cache reads, and the cache-write rate falls back to the cache-read rate | `catalog.json:138-150` (`grok-4.6`), `:48-63` (`claude-sonnet-5-5`) | **Possible, only when the session model is `grok-4.6`.** Not reachable from a stock Anthropic session; `grok-4.6` is never an automatic target (`claude-opus-5-5`, the first frontier row, is at `:64-79`) |
| 4 | **Antigravity small equals mid.** `flash_lite` and `flash` are both 0.075/0.30, so a mid-to-small downshift saves exactly $0 while still accepting the quality risk of the smaller tier. `pro` is 1.5/6 | `catalog.json:460-471` (`flash_lite`), `:472-483` (`flash`), `:484-495` (`pro`) | **No overspend, no saving.** The decision reports `SavingsRatio` 0 (`catalog.go:490` returns 0 when the target is not cheaper). Antigravity has no token source anyway, so no dollar is ever attached. The identical price pair is suspect data, not verified here against Google's price page |
| 5 | **OpenCode entries carry no prices at all.** All seven entries are unpriced: six carry a `-free` suffix (`deepseek-v4.1-flash-free`, `mimo-v2.6-flash-free`, `glm-5.3-flash-free`, `beam-free`, `longcat-2.5-preview-free`, `nemotron-3-ultra-free`) and `big-pickle` does not. `HasModelPrices` requires input and output above zero (`internal/telemetry/cost.go:28-30`), so `real_saved_usd` and `actual_cost_usd` are always 0 for OpenCode, which is "unknown", not "free" | `catalog.json:568-611` (section comment at `:568`, entries `:570-611`); `internal/telemetry/cost.go:28-30` | **Unknown, not zero.** A downshift cannot be priced either way, so neither saving nor overspend is visible. `big-pickle`, the only entry without a `-free` suffix, may be a paid model; the catalog gives no price |
| 6 | **Cursor entries have no cache prices.** No Cursor entry has `cache_read_cost_per_1m` or `cache_write_cost_per_1m`. `CostUSD` bills cached tokens at the input rate (`internal/telemetry/cost.go:32-56`), an upper bound that overstates cost for cache-heavy children. Cursor also records no tokens today, so this bites only when a token source lands | `catalog.json:152-354` (every Cursor entry; none sets a cache field), `internal/telemetry/cost.go:39-56`; catalog note `_doc8` at `:8` | **Distorts the saving, not the sign, in most pairs**: both sides are inflated, but by different shares (a model with a 0.1x cache read gains more from cache than one without). A cheaper-per-input-token model can look pricier once cache is priced correctly, and the reverse |
| 7 | **Haiku 5.5 stored at its low-volume price.** `claude-haiku-5-5` is 0.1/0.5 up to 100k prompt tokens and 0.5/2.5 above; the catalog stores the first. Above 100k the gap to the frontier model is about 87%, not about 97% (`_doc6`). It is priced on its exact id and is never the automatic small target | `catalog.json:34-47`, note `_doc6` at `:10` | **No** for routing (never a target). It **understates** the cost of a child already running it with a long context, so `real_saved_usd` against it is overstated for those spawns |
| 8 | **Stale prose claiming `gpt-6.1-sol` is unpriced.** It has list prices since #190 (2/10, cache read 0.10, write 2.50) | `docs/reference/harness-matrix.md:144-146` and `docs/guides/evidence.md:158` said otherwise; this change corrects both | Not a money risk; it made the audit read as if an unpriced model existed in the codex row |

Not anomalies, checked: KiroCrew is monotonic across all three pairs (`claude-haiku-4-5` 1/5, `claude-sonnet-5-5` 2/10,
`claude-opus-5-5` 4/20), and so is the Claude Code Anthropic ladder and the Cursor automatic ladder (1/5, 3/15, 5/25).
The inversions above appear only through non-default session models (1 to 3) or equal prices (4).

### Proposals, not applied

1. **A price guard on the shared downshift path, not a catalog edit.** One check in `core` (for example in `Check`,
   `internal/core/selfcheck.go`) holding a downshift whose target costs at least as much as the current model when both
   are priced (`from.InputM + from.OutputM <= to.InputM + to.OutputM`), with a reason code. That serves every harness
   by the layering rule and removes the overspend for anomalies 1 to 3 whatever the data later contains. It needs a
   regression test on the codex pair. This is a Go change owned by the fullstack front, so it is only proposed here.
2. **A catalog coherence test per harness**: for each harness, automatic small, mid and frontier targets must be
   non-decreasing in input plus output price, and a documented exception list for the rest. `go test
   ./internal/catalog` already checks cache multipliers and missing prices (`docs/reference/prices.md`); tier order is
   not checked.
3. **Re-verify the codex row** (`gpt-6-sol` 2/10 against `gpt-5.6-terra` 2/12) against the vendor page before anything
   else: if the data is right, the codex mid tier is not a cost tier at all and `gpt-6-luna` is the only real downshift
   target on that harness.
4. **OpenCode: report tokens by tier, not dollars**, until a price exists; print "unpriced" instead of $0.
5. **Cursor: add cache prices from the vendor, or label any Cursor dollar figure an input-rate upper bound** when a
   token source exists.
6. **Antigravity: verify `flash_lite` against `flash`**; if equal, say a mid-to-small move there saves $0.

## What to read on each harness

| If you run | Trust | Do not read as |
|---|---|---|
| Claude Code on an API key | Observed child tokens, estimated dollars at list price, refuted rewrites excluded | An invoice: parent session tokens are not in it |
| Claude Code on Pro, Max or Team | Observed child tokens by model and tier; the number of spawns moved to a cheaper or pricier model | Dollars saved: there is no per-token bill to save against |
| Cursor | Decisions and rewrites emitted; model changes are unconfirmed on usage-based plans and discarded on Free and legacy Pro | Any token or dollar figure: none is recorded |
| Codex | Decisions, effort and model emitted; honored inferred | Any token or dollar figure: none is recorded; the catalog's frontier-below-mid order (anomaly 1) is unresolved |
| OpenCode | Decisions only, beta | Any figure; no price, no tokens, no live spawn |
| Antigravity, KiroCrew | Decisions, one observed child on Antigravity, one block and retry on KiroCrew | Any token or dollar figure |

Tokens rerouted, the metric that counts the tokens of usage records linked to applied, non-refuted rewrites by harness
and requested-to-ran model pair, is defined in
[evidence.md](../guides/evidence.md#b-saving-estimated-by-telemetry) as rung B evidence. Its
fields and command output are documented in [stats-export.md](stats-export.md) when the implementation lands.
