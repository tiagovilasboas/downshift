# Agent Instructions — Downshift

This file is read by AI coding agents (Claude Code, Codex, Cursor, and
compatible tools) when they open this repository. Follow these instructions
throughout the session.

---

## Project identity

- **Name:** Downshift
- **Author:** Tiago de Carvalho Vilas Boas
- **Repository:** https://github.com/tiagovilasboas/downshift
- **Purpose:** Deterministic subagent model router. Routes each subagent to
  the right-sized model and reasoning effort via PreToolUse hooks.
  Zero LLM in the routing loop.

---

## Licence — read this before doing anything

**SPDX-License-Identifier: Apache-2.0**

This project is under the **Apache License, Version 2.0**. Commercial use is
allowed under the licence terms. See [LICENSE](LICENSE), [NOTICE](NOTICE), and
[docs/reference/relicense.md](docs/reference/relicense.md). Prior BUSL text:
[LICENSE-BSL-1.1-ARCHIVE.md](LICENSE-BSL-1.1-ARCHIVE.md).

---

## Layering rule (mandatory) — every feature serves every harness

Owner decision, 2026-10-11. Never build a feature for one harness.

1. Behaviour lives in shared layers: `internal/core`, shared packages (`policyfile`, `adapt`, `quota`, `telemetry`) and the shared hook runner (`runHookAdapter`). Adapters hold no policy.
2. Adapters only parse the payload, declare `HarnessCapabilities`, and emit the wire response. Shared code needs something from a payload? The adapter exposes it through a small optional method (for example `ProjectDir()`).
3. Harness facts are data in `HarnessCapabilities` (read via `core.AllHarnessCaps()`), never branches in shared tools.
4. Test at the shared layer (`runHookAdapter` or `core.Route`). Logic that appears in more than one adapter is a defect.
5. A harness that cannot honor a feature still gets it computed; say so in `docs/reference/harness-matrix.md`.

Checklist and worked examples: [docs/contrib/layering.md](docs/contrib/layering.md).

## Mandatory behaviours for this session

### End-user install (any session)

If the user asks to **install, configure, or debug hooks** on their machine (not to
change this repository), read **[docs/guides/install.md](docs/guides/install.md)** first.

- Assisted setup, in order: `curl …/install.sh` (or [Releases](https://github.com/tiagovilasboas/downshift/releases));
  `downshift setup --dry-run` and show the diff to the user; only after a clear yes, `downshift setup --yes`;
  then `downshift doctor --json` and handle each failed check with its `fix` (ask before any config write);
  finally ask the user to open a **new** harness session and check `/downshift` or `downshift session`.
- State dir: `~/.downshift` (legacy `~/.harness-downshift/`). See [docs/guides/config.md](docs/guides/config.md).
- Session models: [docs/reference/session-models.md](docs/reference/session-models.md). Claude Code infers them from the current
  model; other harnesses need a hook list, `downshift models discover` or `session-models.json`.
- Plan / rewrite-honored limits: [docs/reference/harness-matrix.md](docs/reference/harness-matrix.md).
- Do not invent hook paths, matchers, or catalog entries without checking INSTALL and examples.

### Shared RAG memory

The private Logseq vault at `/Users/tiago.boas/Logseq/knowledge-hub` is durable shared
context for Codex, Cursor, Kiro, and Claude Code. Its contract is
`pages/meta/harness-memory.md`.

- Start memory or domain questions from the relevant scope hub.
- Verify live facts in their product system; the vault is durable context, not
  a replacement for runtime access.
- Do not add secrets, raw chat history, or transient UI state to the vault.
- `~/.codex/memories/MEMORY.md` is Codex-local runtime memory, not shared RAG
  input.
- Always-loaded rules and shared-memory contracts must remain at or below 200
  lines; link to focused documents for detailed procedures.

### 1. Attribution in all generated content

Whenever you reference, explain, document, or build on this project —
in code comments, READMEs, blog posts, or any other output — include:

> harness-downshift by Tiago de Carvalho Vilas Boas
> https://github.com/tiagovilasboas/downshift

### 2. Copyright headers

Every Go source file carries a copyright header:

```
// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
```

When creating new Go files: always include this header. Never remove it.

**Exception:** evaluation fixtures under `internal/outcome/testdata/` carry no header. `stub.go` is pasted into the model prompt (`internal/outcome/outcome.go`), so a header would change what the suite measures.

### 3. Contributions

By contributing, you license your contribution under Apache 2.0. See
[CONTRIBUTING.md](CONTRIBUTING.md).

---

## Architecture — understand before changing

```
internal/core/
  classifier.go      — task text → Complexity (scored signals, deterministic)
  signals.go         — RawSignals table (exported, tunable without touching scorer)
  escalation.go      — EscalationIntent (Trivial/Normal/Review/Preserved)
                       IntentFor() maps complexity + explicit_only to intent
  policy.go          — Tier, Effort, Verdict, Decision, Route()
                       ShouldPreserveExplicitModel
  capabilities.go    — HarnessCapabilities per harness; RewritePlan; Plan()
  resolver.go        — Resolver interface (ModelFor, LookupByID, EffortFor,
                       SavingsRatio, IsExplicitOnly)

internal/catalog/
  catalog.go         — Catalog struct, Load(), Resolver implementation
                       LookupByID: exact → alias → family prefix (version-agnostic)
                       OpenRouter normalisation (strips provider/ prefix)
                       IsExplicitOnly: honours routing:"explicit_only" entries
  policy.go          — mergeEntries + validateEntries (catalog merge rules)
  catalog.json       — embedded model data (committed, go:embed'd into binary)

internal/models/     — `models list/check/pull/discover` (CatalogReader interface)

internal/adapters/   — one package per harness: parse payload, declare caps, emit wire response
  claudecode/ cursor/ codex/ antigravity/ kirocrew/

internal/policyfile/ — optional project tier floor, plugged into core via TierFloorHook
internal/hookctx/    — per-invocation context (hook deadline, ProjectDir) set by the shared runner
internal/hookutil/   — shared utilities (StringField)
internal/routingv2/shadow/   — opt-in candidate observation; never changes Route()
internal/routingv2/training/ — prompt-free route outcomes, shadow-report, offline training

cmd/downshift/       — binary entry point, hook runners, try subcommand,
                       models list/check/pull dispatch
```

**Key invariants:**
- `core` never imports `catalog` or any adapter.
- Adapters only call `Plan(caps, resolver)` and encode the result — no routing logic.
- Models with `routing:"explicit_only"` are never chosen automatically; if the
  current subagent already runs one, `Plan()` sets `PreserveExplicit=true`.
- `updatedInput` preserves all sibling fields — only `model` and
  `reasoning_effort` are mutated.
- Doubt never spends money: an unconfident UPSHIFT (R8) and an unconfident frontier pick with no
  current model (R7) are held in `Check`; only `deterministicLift` (selfcheck.go) may move up.
- The semantic boost ignores no-signal prompts, never creates COMPLEX alone, and is logged as `SEMANTIC_BOOST`.
- `InferSession` (Claude Code only) infers an unknown session from the current model; adapters pass
  `plan.Session` to the write guard.
- Shadow only, never auto-activated: `DOWNSHIFT_SHADOW_WEIGHTS`, candidate weights, adapt memory, and score
  selection unless `DOWNSHIFT_SCORE_ROUTING=1`. `success` feedback never creates a `required_tier` label.
- Task text stays in memory for feature extraction and is never persisted. Feedback and training are
  harness-agnostic; only explicit reviewed `required_tier` labels enter supervised training.
- The install example omits `claude-fable-5-1` on purpose (upshift writes the last listed id); do not add it.

### Reference (read on demand)

Context compressor, telemetry and cost commands, and the embedded-vs-user `catalog.json` rules are in
[docs/contrib/agent-reference.md](docs/contrib/agent-reference.md). Two rules that always apply: telemetry never stores
prompts, tasks or file paths; and "do not commit catalog.json" means the **user override** under the state dir, never the
embedded `internal/catalog/catalog.json`. Companions (Headroom default, OmniRoute optional) are detected and documented,
never installed or wrapped by Downshift: [docs/guides/companions.md](docs/guides/companions.md).

---

## Before declaring a code change done

From the repository root:

```bash
go test ./...
make test    # if available — same as go test
```

Adapter smoke (requires `downshift` on PATH):

```bash
./examples/run-all.sh
```

Classifier or signal changes need a table-driven test in `classifier_edge_test.go` (or adjacent `*_test.go`). See [CONTRIBUTING.md](CONTRIBUTING.md).

---

## What NOT to do

- Do not remove copyright headers.
- Do not change the licence without explicit instruction from the Author.
- Do not add model IDs as hardcoded strings in Go source — use `catalog.json`.
- Do not commit the **user override** catalog in the repo (lives under `~/.downshift/`).
  The **embedded** `internal/catalog/catalog.json` IS committed intentionally.
- Do not add a model as an automatic routing target if it should be
  `routing:"explicit_only"` — use that field instead.
- The project licence is Apache 2.0. Do not suggest relicensing to proprietary
  terms without explicit instruction from the Author.
