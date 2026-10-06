# Architecture — Downshift

High-level view for adopters and contributors. Implementation details live in
code and [CONTRIBUTING.md](../CONTRIBUTING.md).

Downshift by Tiago de Carvalho Vilas Boas  
https://github.com/tiagovilasboas/downshift

---

## What Downshift is today

A **single Go binary** (`downshift`) that runs as a **harness hook** (stdin/JSON).
It classifies task text, applies policy and catalog rules, and returns an
**updated tool input** with `model` (and sometimes `reasoning_effort`) rewritten.

- **No network** on the routing hot path
- **No LLM** chooses the route
- **No HTTP proxy** in the default install (future optional mode)

---

## Request flow (hook)

```text
Harness (Claude Code, Cursor, Codex, …)
    │  PreToolUse JSON on stdin
    ▼
Adapter (per harness: field names, session models, effort)
    │  task text + current model + capabilities
    ▼
core.Plan / Route
    │  classify → escalation intent → policy → catalog Resolver
    ▼
Adapter encodes response (updatedInput / updated_input)
    │  stderr: human-readable decision + feedback id
    ▼
Harness spawns subagent with (hopefully) rewritten model
    │
    ▼
Optional PostToolUse / SubagentStop → telemetry (usage, honor)
```

![Architecture](../docs/img/architecture.svg)

---

## Module boundaries

| Package | Responsibility |
|---------|----------------|
| `internal/core` | Classifier, policy, escalation, session guards — **no** catalog imports |
| `internal/catalog` | Embedded `catalog.json`, user override, `Resolver` |
| `internal/adapters/*` | Harness-specific I/O only; call `core.Plan` |
| `internal/telemetry` | Append-only local JSONL; prompt-free events |
| `internal/semantic` | Optional monotonic semantic boost |
| `internal/routingv2/*` | Experimental / shadow classifier and training |
| `internal/outcome` | Executable coding benchmark tasks (CI) |
| `cmd/downshift` | CLI, subcommands, hook dispatch |

**Invariant:** adapters do not embed routing rules; they only map wire formats.

---

## Classifier and policy (where to read more)

Production routing is **deterministic**: scored patterns (+ optional monotonic
semantic boost) → complexity → tier → catalog model. There is **no LLM** on the
hot path. Adapters apply **rewrite** or **policy mode** (KiroCrew); everything
fail-open.

| Topic | Doc |
|-------|-----|
| Signals, confidence margin, tuning, misroutes | [contrib/classifier.md](contrib/classifier.md) |
| Tier / DOWNSHIFT / UPSHIFT table (product) | [pt/03-marcha.md](pt/03-marcha.md) |
| Shadow / v2 (not the hook default) | [CAPABILITY-ROUTER-V2.md](CAPABILITY-ROUTER-V2.md) |

---

## Configuration on disk

Effective directory: `~/.downshift` with legacy `~/.harness-downshift` fallback
— see [CONFIG.md](CONFIG.md) and `downshift doctor`.

Typical files: `catalog.json`, `events.jsonl`, `session-models.json`,
`loop-events.jsonl`, optional weights.

Overrides: `DOWNSHIFT_STATE_DIR`, `DOWNSHIFT_EVENT_LOG`, `DOWNSHIFT_SESSION_MODELS`,
feature flags in [MINILM-SEMANTIC.md](MINILM-SEMANTIC.md).

---

## Experimental systems (not production hook default)

| System | Role |
|--------|------|
| `routingv2` + shadow | Observe candidate classifier; does not change `Route()` |
| `decisionintelligence` | Advisory, monotonic, `Apply: false` |
| `orchestration/` (Python) | Opt-in LangGraph planner; does not pick models |
| Capability Router v2 doc | Long-term design; see [CAPABILITY-ROUTER-V2.md](CAPABILITY-ROUTER-V2.md) |

---

## Further reading

| Doc | Audience |
|-----|----------|
| [session-models.md](session-models.md) | Operators |
| [CLASSIFIER-SHADOW.md](CLASSIFIER-SHADOW.md) | Shadow mode |
| [benchmark/README.md](../benchmark/README.md) | Benchmark discipline |
| [design/router-generalization.md](design/router-generalization.md) | Evaluation honesty |
| [pt/README.md](pt/README.md) | Portuguese doc set |
