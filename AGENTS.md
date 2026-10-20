# Agent Instructions — Downshift

This file is read by AI coding agents (Claude Code, Codex, Cursor, and
compatible tools) when they open this repository. Follow these instructions
throughout the session.

---

## Project identity

- **Name:** Downshift
- **Author:** Tiago Vilas Boas
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
  finally ask the user to open a **new** harness session and check `downshift session`, or in Claude Code `/downshift:status` (plugin) or `/downshift` (manual alias).
- State dir: `~/.downshift` (legacy `~/.harness-downshift/`). See [docs/guides/config.md](docs/guides/config.md).
- Session models: [docs/reference/session-models.md](docs/reference/session-models.md). Claude Code infers them from the current
  model; other harnesses need a hook list, `downshift models discover` or `session-models.json`.
- Plan / rewrite-honored limits: [docs/reference/harness-matrix.md](docs/reference/harness-matrix.md).
- Do not invent hook paths, matchers, or catalog entries without checking INSTALL and examples.

### Native compact works in ANY harness (supported or not)

`downshift compact -- <command>` cuts the output of an allowlisted test/build/
status command (go test, git status, pytest, vitest, jest, tsc, cypress run,
playwright test, next build, phpunit, cargo test, flutter test, dart test,
gradle, maven; the exact forms are in docs/reference/harness-matrix.md) to its PASS/FAIL summary and keeps the real
exit code. It is a plain shell command, so it needs NO hook and NO adapter: it
runs the same on every OS and in every harness, including one Downshift has no
integration for. When a harness lacks a model-routing hook (KiroCrew has only a
binary allow/block preToolUse, no in-place rewrite), compact is still available
by invoking the command directly — the agent wraps the command itself. Tell
users this: an unsupported harness still gets compaction via the command, even
though it gets no automatic model routing. Auto-rewrite of a bare command into
the compact form needs a shell-rewrite hook channel (`CanRewriteShellCommand`,
Claude Code only); everywhere else it is the direct command.

### Shared RAG memory

Maintainers may use a **private** Logseq vault (not in this repository) as durable
shared context across harnesses. Its contract is `pages/meta/harness-memory.md`
in that vault. Do not publish vault paths, employer KB dumps, or secrets in this
repo; see [publishing-boundaries.md](docs/reference/publishing-boundaries.md).

- Start memory or domain questions from the relevant scope hub.
- Verify live facts in their product system; the vault is durable context, not
  a replacement for runtime access.
- Do not add secrets, raw chat history, or transient UI state to the vault.
- `~/.codex/memories/MEMORY.md` is Codex-local runtime memory, not shared RAG
  input.
- Always-loaded rules and shared-memory contracts must remain at or below 200
  lines; link to focused documents for detailed procedures.

### 1. Attribution in all generated content

Public identity is a primordial privacy rule: use only **Tiago Vilas Boas**,
including diagrams, credits, alt text and metadata. Never expand the name from
history or older files.

Whenever you reference, explain, document, or build on this project —
in code comments, READMEs, blog posts, or any other output — include:

> Downshift by Tiago Vilas Boas
> https://github.com/tiagovilasboas/downshift

### 2. Copyright headers

Every Go source file carries a copyright header:

```
// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
```

When creating new Go files: always include this header. Never remove it.

**Exception:** evaluation fixtures under `internal/outcome/testdata/` carry no header. `stub.go` is pasted into the model prompt (`internal/outcome/outcome.go`), so a header would change what the suite measures.

### 3. Contributions

By contributing, you license your contribution under Apache 2.0. See
[CONTRIBUTING.md](CONTRIBUTING.md).

---

## Architecture — understand before changing

Imports point inward. Checked with `go list` on 2026-10-19; the graph, flows and
known gaps are in [docs/design/architecture.md](docs/design/architecture.md).

```
cmd/downshift/       — CLI, harnessRegistry, runHookAdapter, setup, doctor. Blank-imports
                       nbopinion, policyfile, quota, routeadapt (their init registers core hooks)
internal/adapters/   — claudecode cursor codex antigravity kirocrew opencode: parse the payload,
                       declare HarnessCapabilities, emit the wire response. No policy
internal/core/       — Route, Check (R1-R9), PlanSession, HarnessCapabilities, Resolver, EscalationIntent.
                       Imports only classify, fuse, quotamodel, session (and paths, semantic below them).
                       Optional layers plug in through hook vars: MemoryAdjustHook, TierFloorHook,
                       ScoreHook, NBSuggestHook/NBTrivialHook, quota hooks. ClassifyWithSemantic wraps fuse
internal/classify/   — regex signals, language guard, risk floor (large prompts scan categories concurrently). No tier
internal/fuse/       — raise-only semantic boost (imports classify, semantic; not core)
internal/session/    — SessionList and loaders (not core). PlanForSession, RouteForSession,
                       InferSessionFromCurrent and CanWriteSessionID stay in core
internal/quotamodel/ — pure quota types, no I/O. internal/quota/ does the file I/O and registers the hooks
internal/nbtier/ nbopinion/ — naive-Bayes opinion; nbopinion registers the hooks, may import core
internal/catalog/    — embedded catalog.json (prices_as_of, synced weekly), user override, Resolver;
                       Load runs under recover in the hooks (loadCatalogFailOpen)
internal/discovery/ models/ — `models discover/list/check/pull`; setup runs discovery
internal/policyfile/ adapt/ routeadapt/ scoreindex/ — project tier floor, shadow memory, score cache (via hooks)
internal/hookctx/ hookutil/ hookport/ childend/ — hook deadline, field helpers, honor/usage ports, child-end recorder
internal/telemetry/  — prompt-free JSONL (no per-spawn fsync), Summarize (the one savings definition)
internal/server/ sensor/ contextopt/ compressor/ codeintel/ — dashboard, tool-output sensor, compaction, DCI
internal/routingv2/shadow/ training/ — opt-in candidate observation; never changes Route()
```

**Key invariants** (each checked on 2026-10-19; recheck with `go list -deps ./internal/core`):
- Core production sources never import `quota`, `nbtier`, `nbopinion`, `catalog`, `telemetry`, `adapt`, `policyfile`, `scoreindex` or an adapter. Core test files do import `catalog`, `nbopinion`, `nbtier` and `quota`; they are not in the binary. No test enforces this graph yet: it is a manual check.
- Adapters do not choose the tier; they call `core.Route`, then apply the plan core returns.
- `session` does not import `core`. `fuse` does not import `core`. `codeintel` imports only `paths`. Score selection stays in `core` through `ScoreHook`; do not merge it with the naive-Bayes opinion.
- The semantic boost and the naive-Bayes opinion stay in-process and do not exec. The boost ignores no-signal prompts, never creates COMPLEX alone, and is logged as `SEMANTIC_BOOST`.
- Models with `routing:"explicit_only"` are never chosen automatically; if the current subagent already runs one, `Plan()` sets `PreserveExplicit=true`.
- `updatedInput` preserves all sibling fields; only `model` and `reasoning_effort` are mutated.
- Doubt never spends money: an unconfident UPSHIFT (R8) and an unconfident frontier pick with no current model (R7) are held in `Check`; only `deterministicLift` (selfcheck.go) may move up.
- `InferSession` (Claude Code only) infers an unknown session from the current model; adapters pass `plan.Session` to the write guard.
- Shadow only, never auto-activated: `DOWNSHIFT_SHADOW_WEIGHTS`, candidate weights, adapt memory, and score selection unless `DOWNSHIFT_SCORE_ROUTING=1`. `success` feedback never creates a `required_tier` label.
- Task text stays in memory for feature extraction and is never persisted. Only explicit reviewed `required_tier` labels enter supervised training.
- Hooks fail open: 1 MiB stdin cap, 2 s read timeout, 1.5 s deadline, panic and catalog-load panic become an unchanged spawn.
- The install example omits `claude-fable-5-1` on purpose (upshift writes the last listed id); do not add it.
- `hooks/cache-tax/` (no longer loaded by the plugin; separate install), `mods/savvy-progress/` and `mods/filetree/` are vendored third-party mods (MIT, pinned in NOTICE): re-vendor per their READMEs; edit only through the documented local patch (`downshift-local.patch` + README section + NOTICE), then re-vendor and reapply it (owner decision 2026-10-19); they hold no routing logic. `mods/downshift-dock/` and `mods/downshift-theme/` are original Downshift code.

### Reference (read on demand)

Context compressor, telemetry and cost commands, and the embedded-vs-user `catalog.json` rules are in
[docs/contrib/agent-reference.md](docs/contrib/agent-reference.md). Two rules that always apply: telemetry never stores
prompts, tasks or file paths; and "do not commit catalog.json" means the **user override** under the state dir, never the
embedded `internal/catalog/catalog.json`. Companions (Headroom default) are detected and documented,
never installed or wrapped by Downshift: [docs/guides/companions.md](docs/guides/companions.md).
One exception to "Downshift does not wrap commands" (owner decision 2026-10-19): the default-on
`downshift compact` wrapper, product name Compactor (`downshift compactor` is an alias; the plugin registers its Bash hook; `"exec": false` in the config or `DOWNSHIFT_EXEC=0` turns it off) may rewrite an
allowlisted shell command with no shell metacharacters to `downshift compact -- <argv>` (no Bash allow rule is needed; a deny or ask rule that matches the command stops the rewrite), which keeps the exit code
and compacts only clean output. Its policy lives in `internal/execwrap`, never in an adapter; it never wraps a companion.

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

If you touched paths, HOME, symlinks, setup, wrappers or the exec layer, confirm
it is OS-agnostic: no `/`-hardcoded path, no `$HOME`, no POSIX-only wrapper on a
Windows path. The `test-os` CI job runs the OS-sensitive packages on
`windows-latest` and `macos-latest` — it must be green, not just the Linux
`test` job.

---

## What NOT to do

- Do not remove copyright headers.
- Do not change the licence without explicit instruction from the Author.
- Do not add model IDs as hardcoded strings in Go source — use `catalog.json`.
- Do not write OS-specific paths or assumptions. Downshift ships on Windows,
  Linux and macOS. Build paths with `filepath.Join` / `filepath.FromSlash`
  (store `/`-separated, convert per-OS), resolve HOME with `os.UserHomeDir()`
  (never `os.Getenv("HOME")`), and put any OS-specific behavior behind a build
  tag (`_windows.go` / `_unix.go`), as `scorerefresh_*.go`, `procgroup_*.go`
  and `internal/paths/owned_*.go` already do. A `/bin/sh` wrapper is POSIX-only:
  a harness install that must run on Windows needs a `.cmd`/`.ps1` sibling or a
  no-wrapper path. Setup must not auto-write a symlinked target (e.g.
  `~/.kiro/steering` → a versioned repo); `EvalSymlinks` resolves it and behaves
  differently on Windows.
- Do not commit the **user override** catalog in the repo (lives under `~/.downshift/`).
  The **embedded** `internal/catalog/catalog.json` IS committed intentionally.
- Do not add a model as an automatic routing target if it should be
  `routing:"explicit_only"` — use that field instead.
- The project licence is Apache 2.0. Do not suggest relicensing to proprietary
  terms without explicit instruction from the Author.
