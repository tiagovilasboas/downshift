<div align="center">

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="docs/brand/downshift-logo-dark.svg">
  <img src="docs/brand/downshift-logo-light.svg" alt="Downshift" width="320">
</picture>

### A cost control plane for AI coding agents

Downshift compacts verbose command output and routes subagents to right-sized models
— all without an LLM in the loop. Plugs into Claude Code, Cursor, Codex, and KiroCrew.

[![Build](https://github.com/tiagovilasboas/downshift/actions/workflows/ci.yml/badge.svg)](https://github.com/tiagovilasboas/downshift/actions/workflows/ci.yml)
[![License: Apache-2.0](https://img.shields.io/badge/license-Apache--2.0-868A90.svg?labelColor=0A0A0A)](LICENSE)
[![Go 1.27](https://img.shields.io/badge/go-1.27-868A90.svg?labelColor=0A0A0A)](https://go.dev)
[![Release](https://img.shields.io/github/v/release/tiagovilasboas/downshift?include_prereleases&label=release&labelColor=0A0A0A&color=868A90)](https://github.com/tiagovilasboas/downshift/releases)

[Get Started](#get-started) · [Features](#features) · [Documentation](#documentation)

</div>

---

## The Problem

AI coding agents (Claude Code, Cursor, Codex, KiroCrew) spend most of their context
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

That's **2KB of tokens** saying "all tests passed". Your agent pays for every byte.

## The Solution

Downshift compacts that to:

```
✓ PASS · 54 packages · 0.156s
```

**~50 bytes.** Same information. 97% fewer tokens for this command.

---

## Get Started

```bash
# Install
curl -fsSL https://raw.githubusercontent.com/tiagovilasboas/downshift/main/install.sh | sh

# Configure your harness (Claude Code, Cursor, Codex, KiroCrew)
downshift setup

# Verify
downshift doctor
```

That's it. Downshift now compacts output and routes subagents automatically.

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
milliseconds. When unsure, keeps the requested model — doubt never spends money.

### 3. Feedback and Training

Improve routing over time by reviewing decisions and training locally:

```bash
downshift feedback list           # Pending decisions to review
downshift feedback <id> success   # Mark as correct
downshift train --from-events     # Train from your feedback
```

Only features are stored, never prompts. Training is local.

### 4. Local Telemetry

All metrics stay on your machine in `~/.downshift/events.jsonl`. No data leaves
your system. Run `downshift stats` to see what Downshift observed.

### 5. Code Intelligence (opt-in)

Indexes your repository in-process for reverse call graphs and targeted context.

```bash
downshift dci enable
downshift dci status
```

---

## Measured Results (Compaction)

| Command | Before | After | Reduction |
|---------|--------|-------|-----------|
| `go test` (50 packages, all pass) | ~2,000 tokens | ~50 tokens | 97% |
| `git status` (clean working tree) | ~200 tokens | ~20 tokens | 90% |
| `cargo build` (no warnings) | ~1,500 tokens | ~100 tokens | 93% |

These are token counts for specific commands, measured by the compactor. Total
cost-per-task savings depend on your command patterns, model usage, and how
often compaction applies. Run `downshift stats` to see your actual numbers.

### What Downshift measures vs. what it doesn't

Downshift tracks:
- Bytes before/after compaction (tokens estimated as bytes/4)
- Model routing decisions (what was requested vs. what ran)
- DCI injections and latencies

Downshift does NOT track:
- Your actual invoice (that's between you and your provider)
- Cost-per-completed-task (requires end-of-task signals not yet wired)

The `downshift report` command compares routed vs. control groups when you have
enough data (20+ completed tasks per group). Until then, `downshift stats` shows
what was observed.

---

## Commands

### Daily Use

```bash
downshift session              # Current session savings
downshift stats                # Global statistics  
downshift report --days=7      # Routed vs control comparison (needs data)
downshift doctor               # Health check
```

### Inspection

```bash
downshift try "<task>"         # Preview routing decision
downshift watch                # Live session view
downshift config               # Show effective configuration
```

---

## Harness Support

Downshift plugs into your coding agent's hook system:

| Harness | Compaction | Model Routing | Status |
|---------|------------|---------------|--------|
| **Claude Code** | ✅ Auto | ✅ Full | 🟢 Primary |
| **Codex** | ✅ Auto | ✅ Full | 🟢 Production |
| **Cursor** | ✅ Auto | ✅ Full | 🟢 Production |
| **KiroCrew** | ✅ Via steering | ⚠️ Policy mode | 🟢 Production |
| **OpenCode** | ✅ Auto | ✅ Full | 🟡 Beta |

### How Each Harness Works

- **Claude Code** — Native PreToolUse/PostToolUse hooks. Downshift rewrites shell
  commands automatically via plugin. *This is the reference implementation.*

- **Codex** — Same hook architecture as Claude Code. Full compaction and routing.

- **Cursor** — Rules-based integration. Compaction via command wrapper, routing via
  composer rules.

- **KiroCrew** — Steering-based. Agent follows `downshift-compact.md` steering to wrap
  commands. Model routing is policy-mode only (allow/block, no rewrite) because
  KiroCrew's preToolUse hook has no `updated_input` channel.

- **OpenCode** — Plugin-based integration. Currently in beta testing.

Run `downshift setup` to auto-detect and configure all installed harnesses.

---

## Documentation

### Getting Started
- [Quickstart Tutorial](docs/guides/quickstart.md)
- [Configuration Options](docs/reference/config-options.md)

### Features
- [Output Compaction](docs/features/compaction.md)
- [Model Routing](docs/features/routing.md)
- [Code Intelligence (DCI)](docs/features/dci.md)
- [Local Telemetry](docs/features/telemetry.md)

### Reference
- [Supported Tools](docs/reference/supported-tools.md)
- [CLI Reference](docs/reference/cli.md)

### Architecture
- [System Design](docs/architecture/design.md)
- [Compressor Pipeline](docs/architecture/compressor.md)
- [Routing Engine](docs/architecture/routing.md)

### Contributing
- [Development Setup](docs/contributing/development.md)
- [Adding Compressors](docs/contributing/compressors.md)

---

## Privacy & Security

- **Prompts stay local** — task text is evaluated in-process, never sent anywhere
- **Local telemetry only** — counts, hashes, timings in `~/.downshift/events.jsonl`
- **Routing is 100% local** — no network calls for model selection
- **Fail-open** — any error leaves the command/spawn unchanged

---

## Status

**Beta** (v0.1.0-beta.14). Stable for daily use.

---

## License

Apache License 2.0. See [LICENSE](LICENSE).

---

<div align="center">

**Downshift** by [Tiago Vilas Boas](https://github.com/tiagovilasboas)

</div>
