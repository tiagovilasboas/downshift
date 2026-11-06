# OpenRouter support (plan, 2026-10-20)

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

Goal: when a harness sends its model traffic through OpenRouter, Downshift picks a
right-sized OpenRouter model id for each subagent, deterministically, offline on
the hot path. Downshift does not proxy traffic, store a key in a hook, or call an
LLM to decide (ROADMAP non-goals, `docs/guides/when-to-use.md`).

## What exists today

- Catalog strips the first `provider/` segment of an id, so `anthropic/claude-haiku-5.5`
  resolves to the Claude entry (`internal/catalog/catalog.go`, capability matrix
  "Implemented, not validated everywhere"). Ids outside the catalog stay unknown.
- Prices: the weekly `sync-prices` workflow reads the OpenRouter public model list.
- Scores: `openrouter-aa` (Artificial Analysis index through the public list), shadow only.
- Discovery (`internal/discovery`) asks each harness what the account can use; the
  catalog ranks what was discovered and never adds an id the harness did not report.
- OpenCode already carries a `providerID` and `id` on a spawn, and 7 catalog entries.

## Design

1. **Discovery source `openrouter`.** Reads the public `GET /api/v1/models` (no key),
   bounded size and timeout, cached like the other sources, run by `downshift models discover`
   only. An optional account-filtered list uses a key passed to that one command (Keychain
   reference, never printed, never read by a hook).
2. **Resolver keeps the full `provider/slug` for OpenRouter entries.** The prefix strip
   collides: `deepseek/deepseek-v4.1-flash` and the OpenCode id `deepseek-v4.1-flash-free`
   are different models. Strip only when no OpenRouter entry matches the full slug.
3. **Catalog rows `provider: "openrouter"`**, `routing: "explicit_only"` by default: never
   chosen automatically. A user allowlist (`~/.downshift/openrouter.json`: id, tier, max
   effort, `zdr` flag) is what turns a row into a routing target. A discovered id that is
   not in the catalog or the allowlist stays unknown, never guessed from price or index.
4. **Shadow first for cross-provider moves.** Claude on OpenRouter to a DeepSeek, Qwen or
   Kimi model is the claim `docs/guides/evidence.md` marks unproven. Record `would_route`
   (and the score, when mapped) and measure; no rewrite until reviewed labels exist.
5. **Same-vendor moves through OpenRouter can go live first** (`anthropic/…` opus to haiku):
   they map onto existing entries and tiers.
6. **Risk floor is unchanged.** Auth, payments, crypto and private data are never
   downshifted. An allowlist row with `zdr: false` is never a target for a task with any
   risk category, and the OpenRouter-side data policy stays the account's setting.
7. **Telemetry stays tokens only and prompt-free.** One new label: provider.

## Parts (one reason to change, one PR each)

| # | Part | Behaviour change |
|---|------|------------------|
| 1 | Resolver keeps the full slug for OpenRouter entries; collision tests | none without OpenRouter rows |
| 2 | Catalog schema `provider: "openrouter"`, `explicit_only` default, allowlist loader and `docs/guides/config.md` | none until a row is allowlisted |
| 3 | Discovery source `openrouter`, recorded fixture tests, shown by `models discover` and `models list` | none on the hot path |
| 4 | Shadow routing for OpenRouter targets (implemented, shadow only): `gateway_would` on the decision event, `stats --detail` line | records only |
| 5 | Harness wiring facts: OpenCode, Claude Code through the OpenRouter endpoint, Codex custom provider, in `docs/reference/harness-matrix.md` | docs, after live checks |
| 6 | Optional: live rewrite for same-vendor OpenRouter ids behind a flag | visible to users, last |

Left out: cross-provider rewrite by default, key storage, credits and quota, any proxy.

## Not verified yet (check before parts 3 and 5)

- The model id each harness sends when pointed at OpenRouter (Claude Code via
  `ANTHROPIC_BASE_URL`, OpenCode `providerID`, Codex custom provider).
- The shape of the account-filtered models endpoint and what it reports about ZDR.
- Current OpenRouter terms for reading the public list at this cadence
  (`docs/reference/scores.md` already says this is not verified).

## Owner decisions

- **First harness.** Recommended: OpenCode (it already has `providerID` and `provider/id` ids).
- **Allowlist.** Recommended: explicit user file only, no automatic tiering from price or index.
- **Cross-provider.** Recommended: shadow until labels exist (the same gate as economy mode).
