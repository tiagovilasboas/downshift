# README hero — draft for post-rename README

Use **Version A** as the default top of `README.md` after the GitHub rename to `downshift`.  
Versions B and C are alternates for A/B tests or Product Hunt.

Migration context: [RENAME.md](RENAME.md)

---

## Shared badges (update URLs after repo rename)

```markdown
# Downshift

[![Build](https://github.com/tiagovilasboas/downshift/actions/workflows/ci.yml/badge.svg)](https://github.com/tiagovilasboas/downshift/actions/workflows/ci.yml)
[![License: Apache-2.0](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)
[![Go 1.27](https://img.shields.io/badge/go-1.27-00ADD8.svg)](https://go.dev)
[![Status: beta](https://img.shields.io/badge/status-beta%20%E2%80%94%20practical%20testing-yellow)](https://github.com/tiagovilasboas/downshift/releases)
```

---

## Version A — Technical (recommended)

```markdown
# Downshift

**Deterministic model routing for AI workloads.**

Downshift classifies each workload, applies catalog policy, and selects model tier and reasoning effort **before execution**—without an LLM in the routing loop. One Go binary: use it as a **coding harness hook** today; the same core is gateway-shaped for tomorrow.

```
Subagent / request
       ↓
  classify (rules + optional semantic boost)
       ↓
  policy + catalog → tier & model
       ↓
  execute on the chosen model
```

**Works today with Claude Code, Cursor, Codex, Antigravity, and KiroCrew.** Single binary, no runtime dependencies, no network calls for routing, no API keys to pick a tier.

![Downshift routing](docs/brand/downshift-routing-runtime-light.svg)

> **Beta — practical testing phase.** Hooks and adapters are production-minded; harness plans and builds vary in how faithfully they honor `updatedInput.model`. See [Plan compatibility](#plan-compatibility--read-before-installing) before installing.

### Why Downshift?

| | |
|---|---|
| **Right-sized models** | Trivial tasks downshift to small tiers; complex work stays on frontier. |
| **Deterministic** | Scored signals, not an LLM classifier, on the hot path. |
| **Testable** | `downshift try`, [benchmark/REPORT.md](../../benchmark/REPORT.md), optional shadow classifier (`DOWNSHIFT_SHADOW_WEIGHTS`). |
| **Observable** | Local telemetry (`events.jsonl`), feedback with explicit `--required-tier` for training labels. |
| **Harness-agnostic core** | Adapters only encode I/O; routing lives in `internal/core`. |

### Quickstart

```bash
curl -fsSL https://raw.githubusercontent.com/tiagovilasboas/downshift/main/install.sh | sh

downshift try "rename the userId variable" claude-code
# → TRIVIAL → small tier

downshift try "diagnose the race condition in the webhook handler" claude-code
# → COMPLEX → frontier tier
```

**Claude Code hook** (`~/.claude/settings.json`):

```json
{
  "hooks": {
    "PreToolUse": [
      { "matcher": "Task", "hooks": [{ "type": "command", "command": "downshift claude-code" }] }
    ]
  }
}
```

When routing fires, stderr shows the decision (and a feedback id), for example:

```text
downshift: TRIVIAL task → downshift to claude-haiku-4-5 (~75% cheaper)
```

**Keywords:** model routing · LLM cost optimization · agent hooks · Claude Code · Cursor · Codex · deterministic router

---

**Integrations** (docs): Coding harnesses · Catalog & session models · Shadow classifier · Outcome benchmarks

*Downshift by Tiago de Carvalho Vilas Boas · https://github.com/tiagovilasboas/downshift*
```

---

## Version B — Minimalist

```markdown
# Downshift

**Right model. Right tier.**

Open-source, local model router for agents and apps. Classify the workload, pick the tier, run—no LLM in the routing loop.

```bash
curl -fsSL https://raw.githubusercontent.com/tiagovilasboas/downshift/main/install.sh | sh
downshift try "list files in src/" cursor
```

![Downshift](docs/brand/downshift-mark-lightbg.svg)

[Quickstart](#quickstart) · [Plan compatibility](#plan-compatibility--read-before-installing) · [Contributing](CONTRIBUTING.md)
```

---

## Version C — Open source / cost narrative

```markdown
# Downshift

### Stop paying frontier prices for every subagent.

Teams default every spawn to the most expensive model in the session. Downshift intercepts **before** the subagent starts and routes to the right-sized model—automatically, offline, with a single Go binary.

```text
"rename userId in auth.ts"  →  small tier
"rearchitect payment idempotency"  →  frontier tier
```

**Open-source model router** for real agent workflows (Claude Code, Cursor, Codex, and more). Not a hosted gateway markup; not an LLM choosing your LLM.

![Downshift](docs/img/hero.svg)

```bash
curl -fsSL https://raw.githubusercontent.com/tiagovilasboas/downshift/main/install.sh | sh
```

[Why Downshift?](#why-downshift) · [Benchmarks](benchmark/README.md) · [Licence](LICENSE)
```

---

## Mapping from current README

When applying Version A, **keep** existing sections below the hero without rewriting the whole file in one PR:

| Keep as-is (update links only) | Refresh wording later |
|-------------------------------|------------------------|
| Plan compatibility | Replace `harness-downshift` → Downshift in prose |
| dsmon section | State dir: `~/.downshift` after migration |
| Catalog override | Path via state dir |
| Fowler quote block | Keep link; clarify “harness” = integration category |
| Licence | Apache 2.0 |

---

## GitHub repository description (copy-paste)

```text
Deterministic open-source model router — right-sized LLMs per workload. Agent hooks, catalog policy, shadow mode, published eval summary.
```

## One-line tagline options (pick one under H1)

1. **Deterministic model routing for AI workloads.** (default)
2. Route every task to the right model.
3. Stop paying frontier prices for trivial work.
