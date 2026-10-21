<div align="center">

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="docs/brand/downshift-logo-dark.svg">
  <img src="docs/brand/downshift-logo-light.svg" alt="Downshift" width="320">
</picture>

### Compact verbose output. Route subagents to right-sized models.

Downshift compacts verbose command output, routes subagents to right-sized models,
and learns which commands need new compressors — all without an LLM in the loop.

[![Build](https://github.com/tiagovilasboas/downshift/actions/workflows/ci.yml/badge.svg)](https://github.com/tiagovilasboas/downshift/actions/workflows/ci.yml)
[![License: Apache-2.0](https://img.shields.io/badge/license-Apache--2.0-868A90.svg?labelColor=0A0A0A)](LICENSE)
[![Go 1.27](https://img.shields.io/badge/go-1.27-868A90.svg?labelColor=0A0A0A)](https://go.dev)
[![Release](https://img.shields.io/github/v/release/tiagovilasboas/downshift?include_prereleases&label=release&labelColor=0A0A0A&color=868A90)](https://github.com/tiagovilasboas/downshift/releases)

[Get Started](#get-started) · [Features](#features) · [Token Savings](#token-savings) · [Documentation](#documentation)

</div>

---

## The Problem

AI coding agents (Claude Code, Cursor, Codex, KiroCrew, and others) spend most of their context
window on verbose command output that carries little signal:

```
$ go test ./...
ok      github.com/example/pkg1    0.003s
ok      github.com/example/pkg2    0.012s
ok      github.com/example/pkg3    0.008s
ok      github.com/example/pkg4    0.005s
... 50 more lines ...
PASS
```

That's about **2KB of output** saying "all tests passed". Your agent's context window holds every byte.

## The Solution

Downshift compacts that to:

```
✓ PASS · 54 packages · 0.156s
```

**~50 bytes.** Same information. This example is an illustration of the shape of the output, not a
measurement. Measured in bytes on a synthetic test corpus: 82.3% fewer bytes on compacted files
(26 of 248), 23.7% overall
([evidence](docs/evidence/compaction-corpus-2026-10-21.md)). Bytes, not LLM tokens.

---

## Get Started

```bash
# Install
curl -fsSL https://raw.githubusercontent.com/tiagovilasboas/downshift/main/install.sh | sh

# Configure your harness (Claude Code, Cursor, Codex, KiroCrew, ...)
downshift setup

# Verify
downshift doctor
```

That's it. On Claude Code, Downshift now rewrites allowlisted commands and routes subagents
automatically. Other harnesses differ; see [Harness Support](#harness-support).

<details>
<summary>Manual installation</summary>

Download from [Releases](https://github.com/tiagovilasboas/downshift/releases) and
verify with `checksums.txt`. Releases include build-provenance attestation.

</details>

---

## Features

### 1. Output Compaction (default: on)

Wraps allowlisted commands to compress verbose output while preserving exit codes
and failure details.

```bash
downshift compactor -- go test ./...
downshift compactor -- cargo build --release
downshift compactor -- pnpm test
downshift compactor -- git status
```

**Supported tools:** Go · Rust/Cargo · Python (pytest, pip, uv, ruff, mypy) ·
Node.js (npm, pnpm, yarn, bun) · PHP (composer, phpunit, pest) · Git · Docker ·
Kubernetes · Terraform · AWS CLI · and [70+ more](docs/reference/supported-tools.md)

### 2. Subagent Model Routing (zero LLM)

Routes each subagent task to the right-sized model — Haiku for renames, Opus for
architecture — without calling an LLM to decide.

```
$ downshift try "rename the userId variable"
→ small: opus → haiku
why: trivial task, no risk, confident
```

**How it works:** Strict heuristics + FNV word-hash scoring. Local, deterministic,
no model call on the routing path. When unsure, keeps the requested model — doubt never spends money.

### 3. Compressor Learning (auto-discovery)

Observes uncompacted commands and identifies candidates for new compressors.

```bash
$ downshift learning suggestions

Compressor suggestions (highest impact first):

→ 1. npm audit
      Count: 47 runs · Avg: 12.3 KB · Total: 578.1 KB

  2. pip show
      Count: 23 runs · Avg: 1.2 KB · Total: 27.6 KB
```

Runs asynchronously after command completion.

### 4. Code Intelligence (opt-in)

Indexes your repository in-process for reverse call graphs and targeted context.

```bash
downshift dci enable
downshift dci impact MyFunction  # Who calls this?
```

---

## Output Savings

### Compaction (bytes, measured on a synthetic test corpus)

Compaction reduces clean, success-case outputs. Measured in bytes (not LLM tokens) on a
synthetic test corpus of 248 fixtures (Rung B evidence, not real-world traffic), of which 26 were compacted
([evidence](docs/evidence/compaction-corpus-2026-10-21.md)):

| Metric | Value |
|--------|-------|
| Compacted files (26 of 248, clean outputs) | 82.3% fewer bytes |
| Best cases (test runners, all pass) | 90–97% fewer bytes (flutter 97.4%, dart 97.3%, cargo 95.0%, pytest 95.0%, playwright 91.2%) |
| Overall corpus (all 248, includes failures and edge cases) | 23.7% fewer bytes |

*Your savings depend on your command mix. Clean outputs compress well; failures
and edge cases pass through unchanged (by design). Run `downshift stats` to see
your real numbers.*

### Evidence Methodology

Downshift follows a strict [Evidence Ladder](docs/guides/evidence.md):

1. **Bytes measured** — before/after on test corpus, reproducible
2. **Token estimate** — bytes/4, an approximation
3. **Control groups** — `DOWNSHIFT_CONTROL_SHARE` enables A/B comparison
4. **Your invoice** — the only source of truth for cost

We show byte counts and percentages, not dollar figures.

---

## Commands

### Daily Use

```bash
downshift session              # Current session savings
downshift stats                # Global statistics
downshift report --period=7d   # Savings over time
downshift doctor               # Health check
```

### Compressor Learning

```bash
downshift learning suggestions # Candidates for new compressors
downshift learning stats       # Store statistics
downshift learning export      # Raw JSON
downshift learning clear       # Reset
```

### Inspection

```bash
downshift try "<task>"         # Preview routing decision
downshift watch                # Live dashboard
downshift models check         # Verify catalog pricing
```

---

## Harness Support

Downshift was built with Claude Code. The status word says how much was verified on the real
harness, per the [Harness Matrix](docs/reference/harness-matrix.md):

| Harness | Compaction | Model Routing | Status | Setup |
|---------|------------|---------------|--------|-------|
| **Claude Code** | Automatic rewrite | Rewrite, observed honored | Completed | `downshift setup claude-code` |
| **Codex** | `downshift compact` run by the agent | Rewrite emitted, honored inferred | Functional | `downshift setup codex` |
| **Cursor** | `downshift compact` run by the agent | Rewrite emitted, honored unconfirmed | Functional | `downshift setup cursor` |
| **OpenCode** | `downshift compact` run by the agent | Adapter and conformance tests; no live spawn yet | Beta | Plugin in development |
| **Antigravity** | `downshift compact` run by the agent | One observed child on one build | Experimental | `downshift setup antigravity` |
| **KiroCrew** | `downshift compact` run by the agent | Policy mode (allow or block and retry, no rewrite) | Experimental | `downshift setup kirocrew` |

Only Claude Code rewrites a command into its compact form automatically. On every other
harness the compact command works when the agent runs it directly.

### How Each Harness Works

- **Claude Code** — Native PreToolUse/PostToolUse hooks. Downshift rewrites shell commands
  automatically via plugin. Model routing via spawn interception. *This is the reference implementation.*

- **Codex** — PreToolUse hook rewrites the spawn model. The next spawn is the only signal that
  it was honored (inferred, not observed). Compaction is `downshift compact`, run by the agent.

- **Cursor** — preToolUse hook on `Task`. The child model is absent from the transcript, so
  honored is unconfirmed. Compaction is `downshift compact`, run by the agent.

- **KiroCrew** — Steering-based. Agent follows `downshift-compact.md` steering to wrap
  commands. Model routing is policy-mode only (allow/block, no rewrite) because
  KiroCrew's preToolUse hook has no `updated_input` channel.

- **OpenCode** — Adapter and shared conformance suite are in the repository; the forwarding
  plugin is in development and no live spawn is observed yet.

- **Antigravity** — One child observed on desktop 2.21.1 (requested `pro`, executed
  `flash_lite`). Experimental.

Run `downshift setup` to auto-detect and configure all installed harnesses.

---

## Documentation

### Getting Started
- [Installation Guide](docs/guides/install.md)
- [Configuration](docs/guides/config.md)
- [Quickstart Tutorial](docs/guides/quickstart.md)

### Features
- [Output Compaction](docs/features/compaction.md)
- [Model Routing](docs/features/routing.md)
- [Compressor Learning](docs/features/learning.md)
- [Code Intelligence (DCI)](docs/features/dci.md)

### Reference
- [Supported Tools](docs/reference/supported-tools.md)
- [Harness Matrix](docs/reference/harness-matrix.md)
- [CLI Reference](docs/reference/cli.md)
- [Configuration Options](docs/reference/config-options.md)

### Architecture
- [System Design](docs/architecture/design.md)
- [Compressor Pipeline](docs/architecture/compressor.md)
- [Routing Engine](docs/architecture/routing.md)

### Contributing
- [Contributing Guide](CONTRIBUTING.md)
- [Development Setup](docs/contributing/development.md)
- [Adding Compressors](docs/contributing/compressors.md)

---

## Privacy & Security

- **Prompts never leave memory** — task text is evaluated in-process, never logged
- **Local telemetry only** — counts, hashes, timings in `~/.downshift/events.jsonl`
- **No LLM in the routing loop** — routing decisions are local and deterministic
- **Fail-open** — any error leaves the command/spawn unchanged

---

## Status

**Beta** (v0.1.0-beta.14). Not yet stable: the beta label stays until the exit criteria close. See [Beta Exit Criteria](docs/reference/beta-exit.md).

---

## License

Apache License 2.0. See [LICENSE](LICENSE).

---

<div align="center">

**Downshift** by [Tiago Vilas Boas](https://github.com/tiagovilasboas)

Conceptual backbone: [Harness engineering (Fowler)](https://martinfowler.com/articles/harness-engineering.html)

</div>
