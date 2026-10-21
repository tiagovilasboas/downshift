<div align="center">

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="docs/brand/downshift-logo-dark.svg">
  <img src="docs/brand/downshift-logo-light.svg" alt="Downshift" width="320">
</picture>

### Cut your AI coding agent's token bill by up to 40%

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

**~50 bytes.** Same information. **97% fewer tokens.**

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

**Zero performance impact** — runs asynchronously after command completion.

### 4. Code Intelligence (opt-in)

Indexes your repository in-process for reverse call graphs and targeted context.

```bash
downshift dci enable
downshift dci impact MyFunction  # Who calls this?
```

---

## Token Savings

### Measured Results

| Scenario | Before | After | Savings |
|----------|--------|-------|---------|
| `go test` (50 packages, all pass) | ~2,000 tokens | ~50 tokens | **97%** |
| `git status` (clean working tree) | ~200 tokens | ~20 tokens | **90%** |
| `cargo build` (no warnings) | ~1,500 tokens | ~100 tokens | **93%** |
| Mechanical subagent tasks | Opus pricing | Haiku pricing | **~85%** |

### Projected Monthly Savings

Based on typical agent usage patterns:

| Agent Sessions/Day | Estimated Monthly Savings |
|--------------------|---------------------------|
| 10 | $15 - $40 |
| 50 | $75 - $200 |
| 100+ | $150 - $400+ |

*Actual savings depend on your command patterns and model usage. Run `downshift stats`
to see your real numbers.*

### Evidence Methodology

Downshift follows a strict [Evidence Ladder](docs/guides/evidence.md):

1. **Catalog prices** — list prices from providers, dated
2. **Token counts** — bytes observed, tokens estimated (bytes/4)
3. **A/B trials** — controlled comparison with built-in control groups
4. **Invoice verification** — real billing data (your responsibility)

**We never show dollar figures** — only token counts. Your invoice is the truth.

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

Downshift was built with Claude Code and works across all major coding agent harnesses:

| Harness | Compaction | Model Routing | Status | Setup |
|---------|------------|---------------|--------|-------|
| **Claude Code** | ✅ Auto | ✅ Full | 🟢 Primary | `downshift setup claude-code` |
| **Codex** | ✅ Auto | ✅ Full | 🟢 Production | `downshift setup codex` |
| **Cursor** | ✅ Auto | ✅ Full | 🟢 Production | `downshift setup cursor` |
| **KiroCrew** | ✅ Via steering | ⚠️ Policy mode | 🟢 Production | `downshift setup kirocrew` |
| **OpenCode** | ✅ Auto | ✅ Full | 🟡 Beta | Plugin |

### How Each Harness Works

- **Claude Code** — Native PreToolUse/PostToolUse hooks. Downshift rewrites shell commands
  automatically via plugin. Model routing via spawn interception. *This is the reference implementation.*

- **Codex** — Same hook architecture as Claude Code. Full compaction and routing support.

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
- **No network calls** — routing decisions are 100% local and deterministic
- **Fail-open** — any error leaves the command/spawn unchanged

---

## Status

**Beta** (v0.1.0-beta.14). Stable for daily use. See [Beta Exit Criteria](docs/reference/beta-exit.md).

---

## License

Apache License 2.0. See [LICENSE](LICENSE).

---

<div align="center">

**Downshift** by [Tiago Vilas Boas](https://github.com/tiagovilasboas)

Conceptual backbone: [Harness engineering (Fowler)](https://martinfowler.com/articles/harness-engineering.html)

</div>
