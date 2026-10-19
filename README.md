<div align="center">

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="docs/brand/downshift-logo-dark.svg">
  <img src="docs/brand/downshift-logo-light.svg" alt="Downshift" width="320">
</picture>

**Deterministic model router and code-intelligence layer for coding agents: right-sizes subagent models, packs only the relevant code context, and flags wasted prompt cache. Fail-open hooks for Claude Code, Cursor, Codex, and OpenCode. No LLM in the loop.**

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

[![Build](https://github.com/tiagovilasboas/downshift/actions/workflows/ci.yml/badge.svg)](https://github.com/tiagovilasboas/downshift/actions/workflows/ci.yml)
[![License: Apache-2.0](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)
[![Go 1.27](https://img.shields.io/badge/go-1.27-00ADD8.svg)](https://go.dev)
[![Release](https://img.shields.io/github/v/release/tiagovilasboas/downshift?include_prereleases&label=release)](https://github.com/tiagovilasboas/downshift/releases)
[![Status: beta](https://img.shields.io/badge/status-beta-yellow)](#status)

[Quickstart](#quickstart) · [How it works](#how-it-works) · [Domains](#specialized-domains) · [Commands](#commands) · [Harness support](#harness-support) · [Docs](docs/README.md)

</div>

---

## Quickstart

Install the single binary and configure transparent, zero-config hooks across your installed harnesses:

```bash
curl -fsSL https://raw.githubusercontent.com/tiagovilasboas/downshift/main/install.sh | sh && downshift setup
```

`downshift setup` asks, backs up your existing configuration, and merges transparent pre-tool hooks into Claude Code (`~/.claude/settings.json`), Cursor, Codex, and OpenCode. On Claude Code it also installs the Downshift plugin (hooks plus the `cache-tax` mod) and the sibling mods `savvy-progress` (progress bar, `/agents-info` panel of subagents) and `filetree`, and discovers the models of your session. `downshift setup --dry-run` previews changes; `--undo` restores backups; `--no-plugin` keeps the hooks in `settings.json` and skips the mods.

Verify your environment immediately:

```bash
downshift doctor                              # Checklist: binary, hooks, permissions, state
downshift try "rename the userId variable"    # Preview routing decision and rationale
```

> **Epistemic Honesty:** Downshift is fail-open. Any doubt, error, or missing capability leaves the spawn or prompt completely untouched. Model rewrites depend on harness hook support (see [Harness support](#harness-support)).

---

## How it works

Delegating work to subagents is where a model choice can be made per task. Downshift runs in the background through native harness hooks, decides locally and deterministically (no model call), and leaves the spawn untouched when it is unsure:

```
                  ┌────────────────────────────────────────┐
                  │          Parent Coding Agent           │
                  │   (Claude Code, Cursor, Codex, etc.)   │
                  └───────────────────┬────────────────────┘
                                      │
            ┌─────────────────────────┴─────────────────────────┐
            ▼                                                   ▼
 ┌──────────────────────┐                            ┌──────────────────────┐
 │  Pre-Tool Delegation │                            │ User Prompt Submit   │
 └──────────┬───────────┘                            └──────────┬───────────┘
            │                                                   │
            ▼                                                   ▼
┌───────────────────────────────┐                   ┌───────────────────────────────┐
│     Subagent Routing Hook     │                   │     Code Intelligence (DCI)   │
│  - Task complexity classifier │                   │  - AST & symbol index (<10ms) │
│  - Guardrails R1-R9 & floors  │                   │  - Reverse blast radius       │
│  - Right-sizes model tier     │                   │  - Injects compact sketch     │
└──────────┬────────────────────┘                   └──────────┬────────────────────┘
           │                                                   │
           ▼                                                   ▼
┌───────────────────────────────┐                   ┌───────────────────────────────┐
│  Targeted Model Execution     │                   │  Context-Enriched Prompt      │
│  (Haiku / Flash / Luna, etc.) │                   │  (Under defensive boundaries) │
└───────────────────────────────┘                   └───────────────────────────────┘
```

Downshift is four capabilities behind one `setup`, running as native Go hooks inside your harness:

### 1. Deterministic model routing (zero LLM)
- No model call to choose a model: no extra tokens and no extra seconds in the spawn path.
- Strict heuristics plus an in-process FNV word-hash boost that can only raise a tier. The decision is local, in milliseconds.
- **Doubt never spends money.** When the classifier is not confident, the spawn keeps the model it asked for. Mechanical, repetitive work (renames, test runs, listings) is downshifted on the spot, and risk floors protect auth, payment, and secrets.

### 2. Cache hygiene and orchestration awareness (`cache-tax`)
- Prices a cold prompt cache before you pay for it: a cold send on a 50k+ context is held with its estimated rewrite price, and `/keepwarm` keeps the cache alive through a break.
- Tells your own prompts apart from internal traffic (task notifications, peers, coordinators, scheduled triggers), so it never gets in the way of parallel subagent flows.
- Claude Code 2.1.287+ only, installed by `setup` through the plugin.

### 3. Code intelligence (DCI, opt-in)
- Indexes the repository in-process: Go AST, plus TypeScript/JavaScript, Python, Rust, PHP, Java, and C# by pattern.
- Reverse call graph: who calls whom, before the model edits anything (`downshift dci impact`).
- Packs only the affected symbols with BM25 under a strict byte cap, so the model gets the relevant slice instead of the whole tree. Measured so far: latency and bytes, not answer quality. Off by default; `downshift dci enable` turns it on.

### 4. Output compaction (native `contextopt`, in the spirit of RTK and Headroom)
- Recognises noisy terminal output (clean `go test` runs, `git status`, repeated log lines) and compacts only what is unambiguously clean. Errors, stack traces, diffs, grep matches, and non-zero exits stay verbatim.
- **Honest scope:** a Claude Code hook cannot rewrite tool output in flight, so there the native provider measures bytes and saves no tokens. Real in-flight compression on Claude Code needs a proxy such as [Headroom](https://github.com/chopratejas/headroom); Downshift detects and records it but never starts it. The compactor itself is available as a stream (`downshift context compress --exit 0`).

**Practical result.** One install instead of a separate router, a separate cache watcher, and a separate code indexer: `downshift setup` wires the hooks and, on Claude Code, the plugin, the mods, and your discovered models. It is a single Go binary with no daemon. What is proven so far is narrow: up to about 12% list-price saving on mechanical delegated work in a controlled trial, and nothing attributable on natural use; the other capabilities carry their own evidence status in the [capability matrix](docs/reference/capability-matrix.md).

---

## Specialized Domains

Detailed architectural boundaries, deep specifications, and design contracts are maintained in dedicated domain documents:

| Domain | Focus & Responsibility | Specification |
|---|---|---|
| **Subagent Routing** | Deterministic PreToolUse delegation routing, BDI deliberation, invariant guardrails (R1–R9), session catalog resolution | [docs/domains/subagent-routing.md](docs/domains/subagent-routing.md) |
| **Code Intelligence (DCI)** | Zero-runtime AST symbol extraction, reverse call-graphs (blast radius), BM25 ranking, prompt context packaging | [docs/domains/code-intelligence.md](docs/domains/code-intelligence.md) |
| **Context Optimization** | Native context compressor, context sensor, Claude Code session policies (`opusplan`), companion integration | [docs/domains/context-optimization.md](docs/domains/context-optimization.md) |
| **Observability & Telemetry** | Local prompt-free telemetry (`events.jsonl`), automated empirical control groups, child outcome tracking, cost reports | [docs/domains/observability-telemetry.md](docs/domains/observability-telemetry.md) |

---

## Commands

CLI operations are structured by role and execution frequency:

### Everyday (Zero-touch)
Commands for daily setup, diagnostics, and routine reporting:

| Command | Description |
|---|---|
| `downshift setup [--dry-run] [--undo]` | Wire, preview, or restore transparent harness hooks |
| `downshift stats [--days=N] [--dci]` | Summarize routing decisions, estimated savings, and DCI telemetry |
| `downshift doctor [--verbose] [--json]` | Diagnostic health-check for binary, hooks, permissions, and caches |

### Inspection & Simulation
Dry-run simulation and operational inspection utilities:

| Command | Description |
|---|---|
| `downshift try "<task>"` | Simulate routing classification, guardrails, and model selection for any prompt |
| `downshift dci impact [--json] <target>` | Query downstream blast radius and caller dependencies for a symbol |
| `downshift dci inspect [--json] "<prompt>"` | Dry-run code intelligence retrieval, BM25 ranking, and packed context |
| `downshift session --tree` | Interactive tree view of current session's subagents and costs |
| `downshift watch` | Live terminal dashboard of real-time subagent routing events |

### Plumbing & Maintainer
Internal hooks, benchmark harnesses, and catalog tools (*invoked by harness hooks or researchers only*):

| Command | Target / Audience |
|---|---|
| `downshift <claude-code\|cursor\|opencode\|codex>` | Stdin/stdout hook adapters called directly by coding harnesses |
| `downshift dci hook <harness>` | Automated prompt context injection hook runner (1 MiB stdin cap) |
| `downshift dci benchmark` | Measures local Git discovery, AST parsing, and BM25 retrieval latency |
| `downshift context compress [--exit 0]` | Low-level safe stdin compaction stream utility |
| `downshift context benchmark` | Compaction benchmark measuring raw byte reduction on checked-in fixtures |
| `downshift models <discover\|check\|pull>` | Harness model discovery, catalog pricing audit, and remote score refresh |

---

## Harness Support

Every feature is implemented in harness-agnostic shared layers and validated via a shared conformance test suite (`conformance_test.go`). Verified status per harness:

| Harness | Status | Subagent Tool | Hook Mechanism | Observed Evidence |
|---|---|---|---|---|
| **Claude Code** | **Completed** | `Task` / `Agent` | `PreToolUse` → `updatedInput.model` | Observed honored (2026-10-06); transcript tokens; 2026-10-15 A/B trial ([evidence](docs/evidence/claude-code-rewrite-honored-2026-10-06.md)) |
| **Codex** | **Functional** | `spawn_agent` (`multi_agent_v2`) | `PreToolUse` → model + `reasoning_effort` | Inferred from hook responses; conformance suite green ([write-up](docs/evidence/codex-rewrite-honored-2026-10-02.md)) |
| **Cursor** | **Functional** | `Task` | `preToolUse` → `updated_input.model` | Conformance suite green; unconfirmed on Free/legacy Pro plans ([write-up](docs/evidence/cursor-rewrite-honored-2026-10-10.md)) |
| **OpenCode** | **Beta** | `subagent` | Plugin → `subagent_spawn_intent` | Adapter and wire tests in repo; plugin packaging in development ([matrix](docs/reference/harness-matrix.md)) |
| **Antigravity** | **Experimental** | `Task` | Policy mode / native hook | Single-build execution proof (v2.21.1; [write-up](docs/evidence/antigravity-executor-ack.md)) |
| **KiroCrew** | **Experimental** | Native CLI hook | Allow/block retry | Single-build execution proof (v2.29.0; [write-up](docs/evidence/kirocrew-native-hook-2026-10-10.md)) |

---

## Evidence & Truth in Numbers

Downshift follows a strict 5-rung [Evidence Ladder](docs/guides/evidence.md) (from catalog list prices to multi-week invoice verification):

- **Estimates are not invoices**: `stats` and `dsmon` price routing at the catalog's dated list-price snapshot (`downshift models check` compares it with the providers). Treat them as estimates until `downshift report` or your invoice confirms them.
- **Measured in A/B Trials**: In a controlled trial on Claude Code (2026-10-15), mechanical delegated tasks saved ~12% in list-price token costs with 12/12 hidden test suites passing. On complex, natural tasks, guardrails held most spawns at baseline ([full report](docs/evidence/subagent-routing-ab-2026-10-15.md)).
- **Built-in Control Group**: Downshift leaves ~1 in 10 spawns unrouted (`DOWNSHIFT_CONTROL_SHARE=0.10`). `downshift report` statistically contrasts routed vs control arms to measure real cost per completed task and rework rates ([report guide](docs/guides/report.md)).
- **Prompt Privacy**: Task instructions, user messages, and file contents are evaluated in-memory only and never logged to disk or sent across the network. Local telemetry (`.downshift/events.jsonl`) contains counts, hashes, and timings only ([stats export](docs/reference/stats-export.md)).

---

## Mods & Companions

- **Vendored Mods**: Claude Code 2.1.287+ plugins with zero routing logic: bundled `cache-tax` (monitors cold prompt cache prices and offers `/keepwarm`), plus sibling plugins `savvy-progress` (progress bar, `/agents-info` panel) and `filetree`; on Claude Code `downshift setup` installs all three with `claude plugin install` (`--no-plugin` skips them) ([mods guide](mods/README.md)).
- **Companions**: Integrates cleanly beside external proxies like [Headroom](https://github.com/chopratejas/headroom). Downshift records companion presence so savings are never attributed to routing alone ([companions guide](docs/guides/companions.md)).

---

## Documentation

- **Core Documentation**: [Guides Index](docs/README.md) · [Install Guide](docs/guides/install.md) · [Configuration](docs/guides/config.md) · [Architecture](docs/design/architecture.md)
- **Reference & Matrix**: [Capability Matrix](docs/reference/capability-matrix.md) · [Harness Matrix](docs/reference/harness-matrix.md) · [Session Models](docs/reference/session-models.md) · [Complexity Weights](docs/reference/complexity-weights.md)
- **Evidence & Audits**: [Evidence Ladder](docs/guides/evidence.md) · [Subagent A/B (2026-10-15)](docs/evidence/subagent-routing-ab-2026-10-15.md) · [DCI Benchmarks](docs/evidence/dci-mvp-v01.md)
- **For AI Agents**: [AGENTS.md](AGENTS.md) · [llms.txt](llms.txt) · [docs/guides/install.md](docs/guides/install.md)

---

## Contributing

The most valuable contribution is a **misrouted prompt**: run `downshift try "<prompt>"`, note what was chosen versus what you expected, and open an issue. See [CONTRIBUTING.md](CONTRIBUTING.md) and [layering rules](docs/contrib/layering.md).

---

## Status & License

**Status:** Beta (v0.1.0-beta.14). Graduation requirements are tracked in [Beta Exit Criteria](docs/reference/beta-exit.md).

**License:** Apache License 2.0. See [LICENSE](LICENSE) and [NOTICE](NOTICE).

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift · Conceptual backbone: [Harness engineering (Fowler)](https://martinfowler.com/articles/harness-engineering.html).
