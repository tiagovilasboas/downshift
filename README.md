<div align="center">

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="docs/brand/downshift-logo-dark.svg">
  <img src="docs/brand/downshift-logo-light.svg" alt="Downshift" width="320">
</picture>

**A suite for optimizing token use in coding agents: deterministic subagent routing, with the guardrails, floors, quota gate, session view, telemetry, context policy, mods and a companion around it. Conservative under doubt, measured.**

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

[![Build](https://github.com/tiagovilasboas/downshift/actions/workflows/ci.yml/badge.svg)](https://github.com/tiagovilasboas/downshift/actions/workflows/ci.yml)
[![License: Apache-2.0](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)
[![Go 1.27](https://img.shields.io/badge/go-1.27-00ADD8.svg)](https://go.dev)
[![Release](https://img.shields.io/github/v/release/tiagovilasboas/downshift?include_prereleases&label=release)](https://github.com/tiagovilasboas/downshift/releases)
[![Status: beta](https://img.shields.io/badge/status-beta-yellow)](#status)

[Install](docs/guides/install.md) · [Docs](docs/README.md) · [Capability matrix](docs/reference/capability-matrix.md) · [Evidence](docs/guides/evidence.md) · [Harness support](#harness-support)

</div>

## What it is

Coding agents spend most of their money where you are not looking: when the main agent delegates. A subagent inherits the session's most capable model by default, so a rename, a test run or a file listing is billed at frontier price, and a session that delegates a lot multiplies that. Downshift is a single Go binary that sits in the hooks of your coding harness and optimizes that delegated execution: it picks a right-sized model and effort for each subagent, holds back whenever it is not sure, records what each child cost and how it ended, and gives you the tools to measure whether it paid off. **Claude Code is the primary harness** (status: Completed; most complete, most verified). Codex and Cursor are Functional; OpenCode is Beta (adapter and route wire in the repository, plugin wiring in development); Antigravity and KiroCrew are Experimental. All of them run the same core, with the limits in the [matrix](docs/reference/harness-matrix.md) and the tiers under [Harness support](#harness-support).

Model routing is the main lever today, not the whole product. The metric that matters is **cost per task completed with acceptable quality**; tokens are a proxy. Everything is deterministic (no LLM in the routing loop), prompt-free (task text never leaves memory or reaches a log), and fail-open (any doubt or error leaves the spawn exactly as the harness asked).

## The suite at a glance

One binary, one state directory, every piece computed for every harness. Status words are the ones the [capability matrix](docs/reference/capability-matrix.md) uses; each row there names the code, test or dated evidence behind it.

| Piece | One sentence | Status |
|---|---|---|
| **Deterministic subagent routing** | A `PreToolUse` hook classifies the task and rewrites only that subagent's model and effort from the session's own list. | Implemented; honored on Claude Code (observed), inferred on Codex, unconfirmed on Cursor |
| **Guardrails: doubt never spends money** | R1 to R9 hold unconfident downshifts, unconfident upshifts and unconfident frontier picks; risk categories keep a floor; a guessed baseline reaches only the small tier. | Implemented, invariant tests (`cost_safety_invariants_test.go`) |
| **Project tier floor** | `.downshift/policy.json` raises a minimum tier by class or risk for a repository; it never lowers one. | Implemented, every harness |
| **Quota and credit gate** | A hook mark or native usage snapshot holds a rewrite to a model you have no credit for; stale snapshots stop holding after 1 h. | Implemented, not validated everywhere ([quota](docs/guides/quota.md)) |
| **Session view and statusline** | `downshift session --tree`, `/downshift:status` (plugin) or `/downshift` (manual alias), `downshift watch`, a stderr line per spawn and a statusline with quota windows. | Implemented; statusline Claude Code only ([session view](docs/guides/session-view.md)) |
| **Telemetry and cost reports with a control group** | Prompt-free `events.jsonl`, tokens and catalog-priced cost per child, about 1 in 10 spawns left unrouted, `downshift stats` and `downshift report`. | Implemented; usage on Claude Code only; estimates, not an invoice |
| **Context policy** | `setup` sets `opusplan`, plan mode and Claude Code's You should know mod for keys you have not set; the native compressor observes bytes. | Implemented, limited: Claude Code session policy only ([Claude Code guide](docs/guides/claude-code.md)) |
| **Code Intelligence (DCI)** | Structural context optimization: discovers code via Git, extracts multi-language symbols (Go AST, TS/JS, Python), builds local BM25 inverted index, abstains on chatter, packages compact sketches (default 3,500 bytes, configurable 512–16,384) with prompt-injection defense. | Implemented (v0.2); multi-harness hook support (`claude-code`, `cursor`, `opencode`, `codex`, `generic`) ([docs](docs/design/code-intelligence.md)) |
| **Mods** | cache-tax bundled in the plugin; savvy-progress and filetree as sibling plugins of the marketplace; downshift-router as an opt-in prototype. | Shipped, vendored and pinned; router Experimental ([mods](mods/README.md)) |
| **Companions** | Headroom (default compression proxy) is detected and recorded, never installed or wrapped. | Documented ([companions](docs/guides/companions.md)) |
| **Evidence ladder** | Five rungs from list price to cost per completed task, each figure quoted from the file that holds it. | Rung D reached once (2026-10-15); rung E not measured ([evidence](docs/guides/evidence.md)) |

## What it does

| Capability | Status | Where to read |
|---|---|---|
| **Right-size each subagent.** Classify the task, pick small / mid / frontier and a reasoning effort, rewrite only that subagent's model from the session's own list. | Implemented. Rewrite observed honored on Claude Code (2026-10-06); inferred on Codex; unconfirmed on Cursor. The two Experimental harnesses are listed under [Harness support](#harness-support). | [How it works](#how-it-works) · [harness matrix](docs/reference/harness-matrix.md) |
| **Conservative under doubt.** Unconfident upshifts and unconfident frontier picks are held; a guessed baseline downshifts only to the small tier; unsupported languages and risky wording clear confidence. | Implemented, with invariant tests (R1 to R9). | [architecture](docs/design/architecture.md) · [complexity weights](docs/reference/complexity-weights.md) |
| **Guardrails for risky work.** Secrets, payments, auth, destructive operations keep a floor; a project can require a minimum tier in `.downshift/policy.json`. | Implemented, every harness. | [config](docs/guides/config.md#project-tier-floor-optional) |
| **See every decision.** A stderr line per routed spawn, a statusline, `downshift session --tree` or `/downshift:status` (plugin) or `/downshift` (manual alias), `downshift watch`, `downshift doctor` with a fix per failure. | Implemented. | [session view](docs/guides/session-view.md) |
| **Know what each child cost and how it ended.** Tokens and catalog-priced cost per subagent, end status (`completed`, `error`, `interrupted`, ...), respawn-after-failure hint. | Implemented on Claude Code (usage from the child transcript). Codex and Cursor end parsers exist but are unverified and carry no usage; Antigravity and KiroCrew record no child end. Estimated from list prices, not an invoice. | [session models](docs/reference/session-models.md#claude-code-real-usage) |
| **See whether the child checked its work.** `verification-report` groups cost and the last recognized test/lint/type-check result by tier. | Implemented on Claude Code. `passed`, `failed`, `none` and `unrecorded` are sensor observations, not task acceptance. | [verification evidence](docs/guides/verification.md) |
| **Measure instead of believe.** A built-in control group (about 1 in 10 spawns unrouted), `downshift stats` for estimates, `downshift report` for routed versus control cost per completed task with a confidence interval. | Implemented. The strict definition (a child that ended `completed`, with its rework, 20 tasks per arm) has not unlocked yet; the 2026-10-15 A/B measured cost per run with an acceptance check. | [evidence ladder](docs/guides/evidence.md) · [benchmark protocol](docs/guides/benchmark-protocol.md) |
| **Find the next saving.** Skills for setup, read-only audit and a fixed-order cost diagnosis (routing, fixed context, tool output, project policy, misroutes). | Implemented as agent guidance; a skill's own description costs context. | [skills](skills/) · [companions](docs/guides/companions.md) |
| **Context and session policy.** Native observe-only compressor; Headroom detected as companion; `setup` sets `opusplan`, plan mode and Claude Code's You should know mod on Claude Code only for keys you have not set. | Implemented, limited: a hook cannot change tool output in flight; session policy is Claude Code only. | [context optimization](docs/design/context-optimization.md) · [Claude Code guide](docs/guides/claude-code.md) |
| **Code intelligence & context optimization (DCI).** Auto-inject structural context (files, Go/TS/JS/Python symbols, line ranges, literal snippets) on prompt submit across harnesses; abstain on conversational greetings. | Implemented (v0.2, pure Go, zero LLM, <10ms warm p95). | [code intelligence](docs/design/code-intelligence.md) · [evidence](docs/evidence/dci-mvp-v01.md) |
| **Review outcomes before changing routing.** Adapt memory, naive-Bayes second opinions, capability scores and candidate classifiers. | Shadow or opt-in. Observations do not automatically retrain the router or enable a candidate. | [design](docs/design/) |
| **Know the models.** An embedded catalog of 34 entries across the Anthropic, OpenAI, Gemini, Grok and Cursor-native families, with tiers, list prices, aliases and family prefixes; `explicit_only` models are never picked automatically; OpenRouter-style `provider/` prefixes are stripped before lookup. | Implemented (`catalog_test.go`). | [The LLMs it knows](#the-llms-it-knows) · [session models](docs/reference/session-models.md) |
| **Read English and Portuguese.** Weighted regex signals plus an in-process FNV word-hash boost; a language guard clears confidence outside coverage. | Implemented for EN and PT-BR; language detection is heuristic. External MiniLM, E5 and Graphify were removed. | [Languages](#languages) · [classifier](docs/contrib/classifier.md) |
| **Mods and companions.** Three vendored Claude Code mods, one experimental router mod, four recommended mods, one companion. | Shipped (mods); documented (companions). | [Mods](#mods) · [companions](docs/guides/companions.md) |

Status words follow one rule: *Implemented* means code, tests and a dated observation; *not validated everywhere* means the harness or plan has not shown the effect; *Shadow* and *Experimental* never touch a spawn. The full table with evidence per row is the [capability matrix](docs/reference/capability-matrix.md).

## How the pieces connect

The main agent delegates; Downshift recommends a checked model and effort; the
harness executes. Available observations feed local reports and offline review.
Start with the [BDI conceptual view](#bdi-agentic-view-beliefs--intentions), then
follow the [complete execution diagram](#complete-execution-flow). Both distinguish
the router's decision from what the harness actually applies.

Who owns each box, and what Downshift deliberately does not do (it is not an HTTP proxy, it does not rewrite tool results, it does not store prompts): [architecture](docs/design/architecture.md).

## What is proven, and what is not

One controlled A/B on 2026-10-15, one operator, Claude Code, same tasks in a routed and a control arm, judged by hidden tests: on mechanical pieces handed one per subagent (the ceiling case) cost fell about 12% with the same 12 of 12 accepted; on natural tasks most spawns were held and the cost difference is not attributable to routing. A trivial downshift from Opus to Haiku is about 75% cheaper on uncached tokens at list price and about 50% on cache reads, which are most of a child's traffic; how much of your work is trivial and delegated decides what that is worth. Details, limits and the defects those runs exposed: [what the evidence shows](#what-the-evidence-shows) and the [evidence ladder](docs/guides/evidence.md), which separates list-price theory, telemetry estimates, controlled benchmarks and real-task results.

## Quickstart

The integration priorities are **Claude Code, Cursor and OpenCode**. Claude
Code is the most validated integration today; Cursor still needs paid-plan
rewrite proof. OpenCode has a Go adapter and wire tests in the repository,
while its native plugin/installer integration remains in development.

```bash
curl -fsSL https://raw.githubusercontent.com/tiagovilasboas/downshift/main/install.sh | sh
downshift setup --dry-run   # shows the diff per harness found; writes nothing
downshift setup             # asks, backs up, wires the hooks
```

On Claude Code the plugin is the alternative to `setup` (same hooks, the `/downshift:status` command, the skills, an example agent and the bundled [cache-tax mod](hooks/cache-tax/README.md), which prices a cold prompt cache before you pay it and keeps it warm on `/keepwarm`; the binary is still required; use one channel; the marketplace also offers savvy-progress and filetree, one install each, see [mods/](mods/README.md)):

```text
/plugin install downshift --marketplace tiagovilasboas/downshift
```

Then open a new session and check it: `downshift doctor` (every failure has a fix), `downshift try "rename the userId variable"` (what would be picked, and why), and after a few spawns `downshift stats --days=1` and `downshift session --tree`. Setup also sets `"model": "opusplan"` (Opus plans, Sonnet executes), plan mode and Claude Code's built-in "You should know" mod (`cc-plugin-you-should-know@builtin`; each check is a billed background request, `/plugin disable cc-plugin-you-should-know@builtin` turns it off) on Claude Code, only for keys you have not set yourself; `downshift setup --no-session-policy` wires the hooks and leaves all three alone.

> **Beta.** Whether a rewrite is applied depends on the harness and plan, not only on Downshift. Read [plan compatibility](docs/guides/install.md#plan-compatibility--read-before-installing) before installing, and [Harness support](#harness-support) for what has been verified.

## Quickstart in detail (Claude Code)

1. **Install.** Use the one-liner above (macOS / Linux). `go install` and Windows are in [install.md](docs/guides/install.md).
2. **Preview, then apply.**
   ```bash
   downshift setup --dry-run   # prints the settings diff; writes nothing
   downshift setup             # asks, backs up your settings, then wires the hooks
   ```
   `setup` merges the hooks into `~/.claude/settings.json` using this binary's absolute path, keeps your other keys, and is idempotent. `downshift setup --undo` restores the latest backup. It also finds Cursor, Codex and Antigravity if they are installed. Plugin users skip this step ([plugin install](docs/guides/install.md#plugin-install-claude-code)); `doctor` warns if both channels are wired.
3. **Open a new session.** Subagents it spawns now go through the hook.
4. **Check it.**
   ```bash
   downshift doctor                              # checklist: binary, hooks, state, scores
   downshift try "rename the userId variable"    # what would be picked, and why
   ```

### What you will see

- **A stderr line per routed subagent**, for example `downshift: correlation_id=… TRIVIAL task → downshift to claude-haiku-4-5 (~75% cheaper)`, followed by `downshift: honor=… quota=… usage=…` once those are observed.
- **A statusline**, set by `setup` when none exists. It shows Claude's quota windows and, when data exists, the current session's routing summary.
- **Child endings.** The `PostToolUseFailure` and `SubagentStop` hooks record how each child ended (`completed`, `error`, `interrupted`, ...). `stats` prints them under "How children ended"; error text is never stored.
- **`downshift try`** (real output, trimmed):
  ```text
  assumed current: claude-opus-5-5 (harness default)
  → small: opus → haiku
  why: trivial task, no risk, confident
  ```
- **`downshift stats --days=1`** (real output, trimmed; estimates from catalog prices, not a bill):
  ```text
  Last 1 day · 77 subagents · net saved $15.51 (saved $33.72, cost more $18.21)
  Guardrails held 35 risky moves · frontier rewrites on doubt: 0
  Models: sonnet 39 · opus 29 · haiku 8
  ```
- **`downshift session --tree`**, also `/downshift:status` with the plugin or `/downshift` after the optional manual copy ([install.md](docs/guides/install.md#the-downshift-command-claude-code)). Real output, trimmed. This sample session cost $4.72 more than the no-routing baseline, so savings are not guaranteed:
  ```text
  session · 30 subagents · $55.62 (without downshift: $50.90) · over $4.72
  ├─ subagent 1   opus    …       -  COMPLEX
  ├─ subagent 10  sonnet  ✓   $0.07  MEDIUM, unsure, risk floor
  ├─ subagent 22  haiku   ✓   $0.04  TRIVIAL, confident
  └─ subagent 30  haiku   …       -  TRIVIAL, confident
  ```

### Commands

| Command | Use it to |
|---|---|
| `downshift setup [--dry-run] [--undo]` | wire, preview or undo the hooks |
| `downshift doctor [--verbose] [--json]` | check binary, hooks, state and scores |
| `downshift try "<task>"` | see what would be picked, before any spawn |
| `downshift stats [--days=N] [--dci]` | decisions, estimated savings, and DCI telemetry |
| `downshift session --tree` | this session's subagents as a tree |
| `downshift dci status` | code intelligence mode, cache stats, and health |
| `downshift dci enable [active\|observe] [--budget=N]` | enable automatic context injection and optional byte budget |
| `downshift dci benchmark` | local indexing and retrieval latency benchmarks |

More: `downshift help --all` lists every command by group. `downshift watch` is the live tree and `downshift config` shows every effective setting and where it came from.

<details>
<summary>Manual setup</summary>

Prefer to edit `~/.claude/settings.json` yourself? Follow [Manual setup in install.md](docs/guides/install.md#manual-setup-claude-code). The PreToolUse matcher is `Agent|Task`.

</details>

The session model list is optional on Claude Code: without one, Downshift infers the session from your current model and never upshifts past it. To set the list yourself, see [session-models.md](docs/reference/session-models.md).

## What it is not

An HTTP gateway or proxy (that is LiteLLM's job), a context compressor in the request path (Headroom's; Downshift's own compressor observes), a hosted model marketplace (OpenRouter), or an LLM-based classifier (deterministic signals plus a local semantic score from a word-hash embedder). It acts on subagent spawns inside harnesses that expose a pre-tool hook, and the only network call is the optional, at-most-daily refresh of public capability scores ([scores](docs/reference/scores.md)). Learned tier memory stays in shadow. Comparison with gateways and SDKs: [when-to-use](docs/guides/when-to-use.md).

## How it works
<picture>
  <source media="(prefers-color-scheme: dark)" srcset="docs/img/hook-flow-dark.svg">
  <img src="docs/img/hook-flow.svg" alt="Hook flow: the harness sends a PreToolUse event, Downshift classifies the task, applies policy against the session models, and returns updatedInput with the chosen model, or allows the spawn unchanged." width="720">
</picture>

**Session model stays fixed. Subagent models get routed.**

```
Parent (e.g. Claude Sonnet 5.5) spawns 3 tasks:

  Task 1: "rename a variable"       → trivial    → Haiku (¢ cheaper)
  Task 2: "refactor a module"       → normal     → Sonnet (same tier)
  Task 3: "design a new algorithm"  → complex    → Opus ($ more capable)

Billing and token usage happen at the subagent tier, not the session.
```

**The routing loop:**

1. **Intercept.** The harness fires a `PreToolUse` hook when a subagent is about to start. Downshift reads the task text in memory only; prompts are never stored.
2. **Classify.** Deterministic signals (`internal/classify/signals.go`, scored by `internal/classify/classifier.go`) plus an optional in-process word-hash semantic boost (`DOWNSHIFT_MINILM=0` opts out; policy in `internal/fuse`; `core.ClassifyWithSemantic` is a wrapper) map the task to TRIVIAL, SIMPLE, MEDIUM, or COMPLEX (English and Portuguese vocabulary). A language guard and risk abstention clear confidence on known multilingual failure modes without an LLM in the loop; the language guard and the risk floor live in `internal/classify`. The semantic score only raises, needs a regex signal behind it, and cannot create COMPLEX on its own. Weights and the tier map are in [docs/reference/complexity-weights.md](docs/reference/complexity-weights.md).
3. **Choose.** Policy picks small, mid, or frontier. All adapters resolve session metadata before classification: the session supplies all candidates, exact metadata wins, and the catalog is optional fallback. A declared capability-ordered list can rank new IDs; an unordered availability list cannot ([session-models.md](docs/reference/session-models.md)). Doubt never spends money: an unconfident upshift, or an unconfident frontier pick with no known current model, is held. Only deterministic rules (risk floor, risk abstention, operator tier floor, opted-in NB upshift) may move a task up or hold a rewrite.
4. **Remember, in shadow.** If `adapt-memory.json` has a reviewed hit or miss for that class and feature, Downshift records the tier it would use. The spawn still uses the classifier tier.
5. **Rewrite or stay out.** Downshift returns the new model in `updatedInput`. If anything is unknown or fails, it does nothing and the spawn runs unchanged (fail-open).

Internals and code ownership: [docs/design/architecture.md](docs/design/architecture.md).

### BDI agentic view (beliefs → intentions)

**BDI is a conceptual lens for reading this deterministic router**, not a separate
agent framework or an LLM planner. Beliefs are the available task and session
facts; desires are fixed capability and safety goals; intentions are checked
routing plans. The adapter emits the plan, and the harness owns execution.
Observations are recorded for offline review; they do not automatically retrain
the classifier or change the next spawn.

![Downshift BDI view: task and session facts plus fixed safety goals feed a checked routing plan; the harness executes, and available observations support offline review.](docs/img/downshift-bdi.png)

| BDI | Downshift | Packages / artifacts |
|-----|-----------|----------------------|
| **Perception** | Adapter reads PreToolUse JSON (task, model, session facts) | `internal/adapters/*`, `runHookAdapter` |
| **Beliefs** | Complexity, confidence, risk, language guard, exact session metadata, optional catalog fallback, quota | `internal/classify`, `internal/session` (list loaders), `internal/core/session_resolver.go`, `internal/catalog`, `session-models.json` |
| **Desires** | Match capability to task; hold uncertain changes; respect risk and project floors | `EscalationIntent`, `selfcheck.go` |
| **Deliberation** | `Route` + `Check` (no LLM, no network on hot path) | `internal/core/policy.go` |
| **Intention** | `Decision` + `SafeVerdict` + `PlanForSession` → `RewritePlan` | `Decision`, `RewritePlan` |
| **Action** | `updatedInput.model` / Codex effort / Kiro allow-block | Adapter encode |
| **Feedback** | Honor, usage, child status → telemetry | `internal/hookport`, `internal/telemetry` |

Adapt memory and candidate classifiers observe by default. Score routing and NB
changes require explicit opt-in flags and stay behind the routing guards.

### Complete execution flow

![Nine numbered stages: delegation, local session facts, classification, guardrails, session plan and response, harness execution, observations, reports and offline review. Context tools, Claude Code extras and Headroom sit beside the routing path.](docs/img/downshift-execution.png)

**Read the evidence in three steps:** emitted is a hook response; honored is a
harness observation; usage and verification require their own records. Claude
Code has the most complete evidence. Codex and Cursor carry no child usage today;
Antigravity and KiroCrew remain experimental. See the [harness matrix](docs/reference/harness-matrix.md).

Editable sources: [BDI SVG](docs/img/downshift-bdi.svg) ·
[execution SVG](docs/img/downshift-execution.svg). Code ownership and the
guide/sensor map: [architecture](docs/design/architecture.md#guide-and-sensor-map).

### Languages

The rules cover English and Portuguese, including Portuguese risk terms.
When the language guard detects an unsupported language, including mixed-language
instructions, it drops confidence so uncertain model changes are held. Existing
deterministic safety floors can still raise an undersized model. With no
matching signal, routing also abstains. Detection is heuristic; it does not guarantee
recognition of every foreign-language prompt. See [language coverage and limits](docs/reference/complexity-weights.md#language-coverage).

Two layers ([classifier reference](docs/contrib/classifier.md)):

1. **Deterministic regex classifier (always).** Weighted signals in English and Portuguese vote for TRIVIAL–COMPLEX; risk floor and language guard apply on top.
2. **Word-hash semantic boost (default on).** An in-process FNV word-hash embedder compares the prompt to shipped centroids and may raise a hesitant class; never lower, never on a no-signal prompt, never to COMPLEX alone. Opt out with `DOWNSHIFT_MINILM=0`. A lift is logged as `SEMANTIC_BOOST`.

Historical note: external MiniLM commands, multilingual E5, and Graphify escalation were removed in 2026-10-17 to keep the hook zero-dependency. See [CHANGELOG.md](CHANGELOG.md).

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="docs/img/architecture-dark.svg">
  <img src="docs/img/architecture.svg" alt="Architecture: harness adapters call a harness-agnostic core, which uses a model catalog and the session model list to produce a routing decision." width="860">
</picture>

## The LLMs it knows

**Session first, catalog second.** The current session defines the complete
candidate set. Valid exact metadata can describe a model absent from the
embedded catalog. Invalid descriptors reject that ID, and missing prices remain
unknown. The catalog helps supply capability, alias and price metadata; it never
adds a model to the session. See [the session contract](docs/reference/session-models.md#session-metadata-before-catalog-fallback).

Model data lives in the embedded [`catalog.json`](internal/catalog/catalog.json) (version 2, updated 2026-10-16; 34 entries), never in Go source. A user override under the [state dir](docs/guides/config.md) merges on top. Per harness the catalog carries:

| Harness | Families in the catalog |
|---|---|
| Claude Code | `claude-haiku` (small), `claude-sonnet` (mid), `claude-opus` and `claude-fable` (frontier); xAI `grok` (frontier, custom model config) |
| Cursor | Anthropic thinking variants (`claude-4.5-haiku`, `claude-4.5-sonnet`, `claude-opus`, `claude-fable`), `gpt-5.6-sol`, `gemini`, `grok`, `glm`, `muse-spark`, Cursor's own `composer` slugs |
| Codex | `gpt-6-luna` and `gpt-5.6-luna` (small), `gpt-5.6-terra` (mid), `gpt-6-sol`, `gpt-6.1-sol`, `gpt-5.6-sol` (frontier), `gpt-6-astra` (explicit only) |
| Antigravity | Gemini aliases `flash_lite` (small), `flash` (mid), `pro` (frontier) |
| KiroCrew | the Anthropic `claude-haiku` / `claude-sonnet` / `claude-opus` families |

Rules that hold for all of them, pinned by `internal/catalog/catalog_test.go`:

- **Lookup is exact, then alias, then family prefix.** A newer version of a known family (say a `claude-opus` id the catalog has never seen) still resolves to its tier.
- **`routing:"explicit_only"` is never chosen automatically.** Two entries carry it today, `gpt-6-astra` (Codex) and `claude-fable-5-1-thinking-high` (Cursor). If the current subagent already runs one, `Plan()` preserves it instead of downshifting. `claude-fable-5-1` on Claude Code is opt-in by listing, not `explicit_only`; the install example omits it on purpose.
- **`provider/` prefixes are stripped for catalog lookup.** OpenRouter-style IDs can resolve by family. IDs absent from the catalog can still route when the session supplies valid capability metadata or a declared capability order. Without that evidence, the spawn is held. Family fallback never invents the new generation's price.
- **Session inference on Claude Code.** Without a `session-models.json`, the session list is inferred from the current model as the catalog ladder below it, so routing only ever moves down from what you are running; `DOWNSHIFT_INFER_SESSION=off` disables it. Other harnesses need a hook-supplied list, `downshift models discover` or the file ([session models](docs/reference/session-models.md)).
- **Prices are a dated list-price snapshot** with cache rates where the snapshot had them; `downshift models check` and `models pull` compare and refresh.

## Mods

Mods are Claude Code 2.1.287+ plugins with a hooks module. They are the one exception to the never-bundle rule (owner decision 2026-10-17): each is vendored byte for byte from a pinned MIT upstream, listed in [NOTICE](NOTICE), holds no routing logic and is never written by `downshift setup`. Procedure and tests: [mods/README.md](mods/README.md).

| Mod | Where | What it does | Install |
|---|---|---|---|
| **cache-tax** | inside the `downshift` plugin ([hooks/cache-tax/](hooks/cache-tax/README.md)) | drops one cold send on a 50k+ context with its estimated rewrite price; `/keepwarm` keeps the cache warm through a break (one small model request per 50 idle minutes, it spends usage); `/cache-tax` prints the card | comes with `/plugin install downshift --marketplace tiagovilasboas/downshift` |
| **savvy-progress** | sibling plugin ([mods/savvy-progress/](mods/savvy-progress/README.md)) | progress bar above the prompt and an `/agents-info` panel of subagents | `/plugin install savvy-progress@downshift` |
| **filetree** | sibling plugin ([mods/filetree/](mods/filetree/README.md)) | a file tree pane showing what Claude reads, writes and commits | `/plugin install filetree@downshift` |
| **downshift-router** (Experimental, off by default) | [mods/downshift-router/](mods/downshift-router/README.md) | routes from the in-process `agent.spawn` event by running the same `downshift claude-code` binary over the same stdin contract; fails open, never denies; a prototype for [can Downshift become a mod?](docs/design/mod-router.md). Do not run it next to the settings hook: each spawn is routed twice (observed 2026-10-17) | `userConfig.enabled` or `DOWNSHIFT_MOD_ROUTER=1` after installing it |

Why savvy-progress and filetree are not inside the plugin: a plugin loads one hooks module, and all three mods hook `session.start` without a matcher, so each needs its own plugin.

**Recommended, not bundled** ([companions guide](docs/guides/companions.md#claude-code-mods-downshift-recommends)): **You should know** (built into Claude Code, off by default; `setup` turns it on unless you set it, each check is a billed background request), **Token Weather** (Anthropic sample: the main session's context window above the prompt, next to Downshift's statusline below it), **Replay Theater** (Anthropic sample: step through the last turn's edits, no model call) and **reflect-mod** (claude-reflect, MIT: corrections become `CLAUDE.md` rules after one small Haiku check). The companion **Headroom** (default, compression proxy, detected and recorded) is in the same guide; Downshift installs, launches or wraps none of them.

## Project tier floor (optional)

A project can require a minimum tier with `.downshift/policy.json`. It only raises a tier, never lowers one; a missing or invalid file is ignored and the spawn routes as before. It applies to every harness and still passes the session allowlist and quota gates. Format, evidence per harness for where the project directory comes from, and limits: [config.md](docs/guides/config.md#project-tier-floor-optional). The `stats` output counts the decisions it raised.

## Public capability scores (optional)

Downshift keeps a local cache of public capability scores (Epoch AI ECI, CC BY 4.0; Artificial Analysis via OpenRouter, local cache only) for the session models of each harness. Hooks refresh it in the background, at most once a day, and never wait for it. `DOWNSHIFT_SCORES=off` turns all of it off. Selection by score is shadow by default and changes routing only with `DOWNSHIFT_SCORE_ROUTING=1`. Sources, licences, privacy, the id map and the telemetry fields: [docs/reference/scores.md](docs/reference/scores.md).

## What the evidence shows

Measured on 2026-10-15, one operator, Claude Code, same tasks and prompts in a routed and a control arm, judged only by hidden tests. Full method, limits and the defects the runs exposed: [docs/evidence/subagent-routing-ab-2026-10-15.md](docs/evidence/subagent-routing-ab-2026-10-15.md). The same day's hardening audit (defects fixed with their regression tests, benchmark before and after, what is still open) is in [docs/evidence/hardening-audit-2026-10-15.md](docs/evidence/hardening-audit-2026-10-15.md).

| Situation | Accepted (routed / control) | Cost (routed / control) | Rewritten down |
|---|---|---|---|
| Real fixes, delegation optional (b2) | 7/8 / 7/8 | $0.65 / $0.77, not attributable to routing | 0 spawns |
| Mechanical pieces, one subagent each (b5): the ceiling | 12/12 / 12/12 | $0.59 / $0.67 (-12%) | 19 of 56 (34%) |
| Real fixes, at least 3 subagents (b6) | 8/12 / 9/12 | $0.75 / $0.92, not attributable to routing | 3 of 34 (9%) |

- **Supported:** Downshift routes, and where it rewrites down the task still passes; it saves up to about 12% on mechanical delegated work (list price, three runs per cell).
- **Not supported:** a saving figure for general use. The 64% of subagents that kept their model in the ceiling case were left alone for lack of classifier confidence, and the estimate in `downshift stats` (list price per decision) is not a measurement.
- **Not measured:** quality beyond the checks, other harnesses, more than one operator. The team protocol in [docs/guides/pilot.md](docs/guides/pilot.md) is how to add data; `downshift report` compares routed and control from your own usage.
- **Not yet reported:** the three-arm comparison of [benchmark-protocol.md](docs/guides/benchmark-protocol.md) (fixed Opus, native `opusplan`, `opusplan` + Downshift). No `docs/evidence/benchmark-opusplan-*.md` exists; until one does, "saves beyond `opusplan`" stays unproven in the [evidence ladder](docs/guides/evidence.md#where-downshift-adds-value-beyond-native-opusplan-and-gateways).

## Harness support

Every feature is built in shared layers and computed for every harness ([layering rule](docs/contrib/layering.md)); the shared conformance suite (`cmd/downshift/conformance_test.go`) drives the same 8 routing scenarios against every registered hook. What differs is how much has been verified on a real harness, so each harness carries exactly one of four status words:

- **Completed.** The rewrite was observed honored on a real child, usage and child end are recorded, and the session policy and plugin ship. Only Claude Code.
- **Functional.** The hook emits the rewrite on a real spawn and passes the conformance suite, but observation is partial: the harness applying the rewrite is inferred (Codex) or unconfirmed (Cursor), and child usage is not recorded.
- **Beta.** First-class integration under active development: the adapter, route wire and shared conformance suite are in this repository and green, while the harness-side plugin and the first live observation are still ahead. Not a production claim. OpenCode.
- **Experimental (proof of concept).** Policy mode or a single observed child on one build. Shipped, with install steps in the guide, but not a basis for a production claim.

| Harness | Status | Subagent tool | Mechanism | Rewrite honored (evidence) |
|---|---|---|---|---|
| **Claude Code** | Completed | `Task` / `Agent`, paid plans | `PreToolUse` → `updatedInput.model` | Yes, observed 2026-10-06 ([write-up](docs/evidence/claude-code-rewrite-honored-2026-10-06.md)); usage from the child transcript ([billing audit](docs/evidence/billing-gap-audit.md)); one controlled A/B ([2026-10-15](docs/evidence/subagent-routing-ab-2026-10-15.md)) |
| **Codex** | Functional | `spawn_agent` (`multi_agent_v2`) | `PreToolUse` → model + `reasoning_effort` | Inferred, not observed, 2026-10-02 ([write-up](docs/evidence/codex-rewrite-honored-2026-10-02.md)); install: [Codex](docs/guides/install.md#install-codex) |
| **Cursor** | Functional | `Task` | `preToolUse` → `updated_input.model` | Unconfirmed; discarded on Free and legacy Pro plans ([write-up](docs/evidence/cursor-rewrite-honored-2026-10-10.md), [gap diagnostic](docs/evidence/cursor-gap-diagnostic.md)); install: [Cursor](docs/guides/install.md#install-cursor) |
| **OpenCode** | Beta | `subagent` | plugin → `subagent_spawn_intent` → `downshift opencode` | [Not observed yet](docs/reference/harness-matrix.md#status-per-harness) |
| **Grok CLI** | Config only | `spawn_subagent` | Config in `config.toml`, not a hook | No hook rewrite |

Plans, caveats and revalidation rules: [harness-matrix.md](docs/reference/harness-matrix.md).

### Experimental harnesses

**Antigravity** and **KiroCrew** are proofs of concept, not supported at the level of the table above: each has one observed child on one build (Antigravity desktop 2.21.1: [native child evidence](docs/evidence/antigravity-executor-ack.md); Kiro CLI 2.29.0: [native hook evidence](docs/evidence/kirocrew-native-hook-2026-10-10.md), block and retry only, no in-place rewrite) and no recorded child end. The adapters, tests and hooks stay in the repository and compute every feature; setup is in [install.md](docs/guides/install.md#experimental-harnesses) ([Antigravity](docs/guides/install.md#antigravity-google-deepmind), [KiroCrew](docs/guides/install.md#kirocrew-policy-mode-through-the-native-kiro-cli-hook)).

## Context optimization & native compression

Downshift includes a built-in, zero-dependency **Native Context Compressor** (`downshift context compress`) and event-driven **Context Sensor** that observes context volume (byte counts, never a rewrite) without altering critical failure traces.

- **Enable persists a flag, not a prompt rewrite.** `downshift context enable` saves the native provider as active, plus the exit-code contract. Claude Code PostToolUse then records `CompressExit` in observe mode, using the tool's real exit code. The hook cannot replace tool output, so the model still sees the original result. Prompts are not compressed. Safe mode on the CLI still requires `--exit 0`.
- **Zero external dependencies:** Pure in-process Go engine; zero Python, Rust, Node or LLM runtime requirements.
- **Fail-open & error preservation:** Safe squelch on verbose successes (`go test`, `git status`, logs); never touches errors, stack traces, or failing suites.
- **Bytes, not a token claim.** `downshift context benchmark` prints the four routing scenarios with `not measured` for cost and accuracy, then a measured byte table from checked-in fixtures (original bytes, reduced bytes, byte savings). The unit is bytes. The table is not tokens and not dollars.
- **Management CLI:**
  ```bash
  downshift context status      # Show optimization status & active provider
  downshift context providers   # List supported optimization providers
  downshift context doctor      # Inspect local compressor & harnesses
  downshift context enable      # Enable built-in native compressor (or specify provider)
  downshift context disable     # Revert configurations back to raw state
  downshift context metrics     # Sensor counts; token fields stay unavailable
  downshift context compress    # Compact stdin only with --exit 0 in safe mode
  downshift context benchmark   # Measured fixture bytes; routing cost stays not measured
  ```

See [the context optimization guide](docs/design/context-optimization.md) for architecture, harness contracts, and failure handling policies.

### Code Intelligence (DCI)

Downshift Code Intelligence (DCI) is a zero-runtime, deterministic, privacy-first subsystem that injects high-signal structural context directly into user prompts across supported harnesses (`claude-code`, `cursor`, `opencode`, `codex`, and `generic`).

- **Deterministic Pipeline:** Discovers files via Git (`discover.go`), extracts structural symbols across Go AST, TypeScript/JavaScript, and Python plus lexical tokens (`parse.go`), builds an incremental inverted index with SHA-256 change detection (`index.go`), scores candidates via field-weighted BM25 (`rank.go`), and packages compact context sketches (`pack.go`).
- **Conversational Abstention:** Greetings ("hi", "bom dia"), gratitude ("thanks"), and general chatter abstain immediately without injecting context, saving token headroom.
- **Budget & Boundary Defense:** Configurable UTF-8 byte ceiling via `--budget` (default **3,500 bytes**, clamped between **512** and **16,384 bytes**) per injection; all context is wrapped in defensive boundary markers (`=== DCI UNTRUSTED REPOSITORY CONTEXT ===`) to prevent prompt injection.
- **Multi-Harness Hook Support:** `downshift dci hook <claude-code|cursor|opencode|codex|generic>` accepts prompts from `"prompt"`, `"task"`, or `"user_message"` (with a 1 MiB stdin cap) and emits `hookSpecificOutput` (plus top-level `additionalContext` on non-Claude harnesses).
- **Zero Ingestion of Secrets:** Files matching `.env*`, `*.key`, `*.pem`, `id_rsa`, and token patterns (AWS, GitHub, Stripe, Slack) are automatically excluded.
- **Zero User Data Persistence:** User prompts, raw code snippets, and conversational content are **never saved to disk or logs**. Only file metadata (relative paths, hashes, term counts, line numbers) is cached.
- **Management CLI:**
  ```bash
  downshift dci status                        # Show active mode, file count, and cache stats
  downshift dci enable active [--budget=3500] # Enable live context injection (or 'observe' for telemetry only)
  downshift dci disable                       # Deactivate DCI (reverts to off)
  downshift dci doctor                        # Check directory permissions, discovery, and latencies
  downshift dci inspect "<msg>"               # Dry-run pipeline for a prompt (tokens, ranks, packed sketch)
  downshift dci benchmark                     # Run local indexing, sync, and retrieval latency benchmarks
  downshift dci hook <harness>                # Run stdin hook for claude-code, cursor, opencode, codex, or generic
  downshift stats --dci                       # View empirical prompts, injection rates, and observed latencies
  ```

See [docs/design/code-intelligence.md](docs/design/code-intelligence.md) for architecture and [docs/evidence/dci-mvp-v01.md](docs/evidence/dci-mvp-v01.md) for measured benchmarks.

### Companions

Routing is one lever; the others live in tools Downshift works next to, never wraps. [docs/guides/companions.md](docs/guides/companions.md) covers each one.

- **Headroom (default companion).** [Headroom](https://github.com/chopratejas/headroom) is a local compression proxy (`headroom wrap claude`, `codex`, Cursor manual). When its binary is on PATH, `downshift context enable` records it and `downshift doctor` shows a `companion` row, so savings are not attributed to routing alone. Downshift never installs, launches or wraps it; `headroom wrap <tool>` is yours to run.
- **Claude Code mods Downshift recommends.** **You should know** (`/plugin enable cc-plugin-you-should-know@builtin`, built in and off by default: a tool-less side agent notes one thing you might have missed above the prompt; each check is a billed request, so the guide says when it pays), **Token Weather** (Anthropic sample [mod](https://github.com/anthropics/claude-code-playground/tree/main/claude-code/mods/token-weather) drawing the main session's context window above the prompt with real figures, while Downshift's statusline shows quota and subagent routing below it), **Replay Theater** (Anthropic sample: step through the last turn's edits one diff at a time, no model call) and **reflect-mod** ([claude-reflect](https://github.com/BayramAnnakov/claude-reflect), MIT: turns your corrections into `CLAUDE.md` rules after one small Haiku check per short prompt). Install steps and the read-before-install note are in the guide.
- **Patterns, not dependencies.** Anthropic's read-only `claude-code-setup` is the contract `downshift setup --dry-run` follows; `task-observer` can collect misroutes for `misroute:` issues. Neither is bundled.

## Cost and metrics

- **Silent overspend alarm.** `downshift stats` prints `Frontier decisions N (rewritten on doubt, no deterministic lift: M)` and a WARNING when M is above zero. The `SEMANTIC_BOOST` reason code and its share show when the local embedder, not a rule, raised a tier. Both are prompt-free.
- **Estimated, not billed.** Routing-only savings use normalized units. With native tokens, `downshift stats --days=7` also calculates catalog-priced costs and a same-token baseline comparison. A [dated audit](docs/evidence/billing-gap-audit.md) matched five linked Claude Code records to native models/tokens, including both positive-saving records. These estimates are not an invoice; the controlled period comparison remains open.
- **Real usage on Claude Code (optional).** Add a `SubagentStop` hook running `downshift claude-code-subagent-stop` to record tokens and cost per subagent from its own transcript. Setup and limits: [session-models.md](docs/reference/session-models.md#claude-code-real-usage).
- **How a child ended (Claude Code, optional).** Add `downshift claude-code-post-tool-use` under `PostToolUseFailure` too. It then records a closed status per Agent/Task child: `completed`, `launched` (background, not a success), `interrupted`, `error`, `stopped` (the child ended, with no outcome claim), or `unknown`. The field is absent when no status applies. The error text is never stored, and this is an observation, not a decision, so it does not change the decision counts. `downshift stats --by-harness` prints a Markdown review table (spawns, applied, held, upshift/downshift, effort and the linked tokens and estimated cost per harness and tier, plus the guardrail that held each spawn) for curating what the router chose; it holds counts only, no prompts. `downshift stats` prints a "How children ended" section when any are present, and the export carries them as `child_status`. When a failed (`error` or `interrupted`) child is followed by another spawn in the same session within 10 minutes, the same section adds a "Respawned after a failure" count. It is a hint only: the next spawn may be unrelated work, so it never becomes a tier label.
- **Cost per session and per subagent.** When priced usage records carry a session or agent id, `stats` shows how many sessions and subagents had cost, the mean and the maximum, and the three costliest sessions (hashed ids, short prefix). It is catalog-priced from recorded usage, an estimate and not a bill, and it reports only: Downshift cannot enforce a budget or a concurrency limit from a hook.
- **Baseline source.** Cost events carry `baseline_source` when a baseline was computed. It is `requested` (the model the harness asked for) or `inferred` (Claude Code only: the configured model, used when the transcript had none, reason code `BASELINE_FROM_SETTINGS`). `downshift stats` says how many priced events rest on an inferred baseline.
- **Local only.** Events go to a local `events.jsonl` in the state directory; prompts are not stored. Export format: [stats-export.md](docs/reference/stats-export.md).
- **Classifier numbers.** Published tier accuracy, model pass rates, and the 2026-10-15 adapt comparison (per-task upper bound versus shared-shape memory) are in [benchmark/report.md](benchmark/report.md). Tune the classifier with `downshift try` and [docs/contrib/classifier.md](docs/contrib/classifier.md). Weights: [docs/reference/complexity-weights.md](docs/reference/complexity-weights.md).

## Documentation

[INSTALL](docs/guides/install.md) · [CONFIG](docs/guides/config.md) · [session models](docs/reference/session-models.md) · [session discovery](docs/reference/session-discovery.md) · [ARCHITECTURE](docs/design/architecture.md) · [complexity weights](docs/reference/complexity-weights.md) · [HARNESS-MATRIX](docs/reference/harness-matrix.md) · [WHEN-TO-USE](docs/guides/when-to-use.md) · [companions](docs/guides/companions.md) · [examples](examples/README.md) · [full index](docs/README.md) · [Português](docs/pt/README.md)

Evidence and method: [subagent routing A/B](docs/evidence/subagent-routing-ab-2026-10-15.md) · [hardening audit](docs/evidence/hardening-audit-2026-10-15.md) · [team pilot](docs/guides/pilot.md) · [hook payload probe](docs/guides/probe.md)

Project: [ROADMAP](ROADMAP.md) · [GOVERNANCE](GOVERNANCE.md) · [beta exit criteria](docs/reference/beta-exit.md) · [remaining tasks](docs/internal/gap-tasks.md)

**For AI agents:** start with [docs/guides/install.md](docs/guides/install.md), [AGENTS.md](AGENTS.md) and [llms.txt](llms.txt).

## Contributing

The most valuable contribution is a **misrouted prompt**: open an issue with the `downshift try` output and the tier you expected. Evidence that a rewrite was (or was not) honored on your harness and plan is just as welcome. See [CONTRIBUTING.md](CONTRIBUTING.md).

Every feature must serve every harness: behaviour in shared layers, adapters only parse and emit. Read [docs/contrib/layering.md](docs/contrib/layering.md) before changing code. How the repository is packaged for an agent harness (memory, skills, hooks, agents, plugin) is in [architecture.md](docs/design/architecture.md#packaging-layers-what-an-agent-harness-loads).

## Status

**Beta (v0.1.0-beta.14).** The adapters ship; leaving beta still depends on harness and plan proof, not only the router. Open items tracked in [docs/reference/beta-exit.md](docs/reference/beta-exit.md): **P1.6** Cursor Pro/Ultra rewrite honor still unconfirmed; **P2.3** no second external operator has shared a week of hook exports; **P3.4** five usage-linked records of the fifty required (2026-10-09 audit, not re-exported since); **P3.7** no filled same-week billing comparison against a provider dashboard; and **P4** still has no new blind classifier set of 200+ tasks never tuned against. Claude Code has an honored rewrite write-up; Antigravity and KiroCrew have limited observed proof. The first routed-versus-control list-price run is [2026-10-15 evidence](docs/evidence/subagent-routing-ab-2026-10-15.md); it is not a billing period and does not close P3.

## License

Apache License 2.0. See [LICENSE](LICENSE), [NOTICE](NOTICE) and [docs/reference/relicense.md](docs/reference/relicense.md).

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

Conceptual backbone: [Harness engineering (Fowler)](https://martinfowler.com/articles/harness-engineering.html).
