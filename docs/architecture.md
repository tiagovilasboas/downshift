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
    │  optional: internal/adapt (adapt-memory.json) adjusts tier from local feedback
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
| `internal/adapt` | Local tier memory (`adapt-memory.json`); `AdjustTier` after classify — **missing file = no-op** |
| `internal/routeadapt` | Wires `internal/adapt` into `core.Route` via `MemoryAdjustHook` |
| `internal/catalog` | Embedded `catalog.json`, user override, `Resolver` |
| `internal/adapters/*` | Harness-specific I/O only; call `core.Plan` |
| `internal/hookport` | Optional honor and usage; a nil func is unobserved, not inferred |
| `internal/telemetry` | Append-only local JSONL; prompt-free events |
| `internal/semantic` | Optional monotonic semantic boost |
| `internal/routingv2/*` | Experimental / shadow classifier and training |
| `internal/outcome` | Outcome-eval library; minimal fixtures in OSS; full suite in downshift-labs |
| `cmd/downshift` | CLI, subcommands, hook dispatch |

**Invariant:** adapters do not embed routing rules; they only map wire formats.

---

## Harness observation port

Every harness hook has four stages: spawn, rewrite, honor, and usage.

Spawn and rewrite stay on `runHookAdapter` plus the per-harness `Handle`.
The adapter reads task text, the current model, and session or quota marks
only when that stdin contains them. Missing fields stay missing.

Honor (the model the child actually ran) and usage (child token counts) are
optional. They go through `internal/hookport`. A nil port func is unobserved:
the binary does not invent a model, a token count, or a note. Absence is not
inference.

Escalation stays in `internal/core`. Adapters do not choose tiers and do not
escalate.

Plug-in rule: a new harness is an adapter package plus one PreToolUse switch
case. Add an honor/usage map entry only when that hook sends the field.

---

## Configuration on disk

Default state directory (migration in progress — see
[rename.md](brand/rename.md)):

- `~/.downshift/` (legacy `~/.harness-downshift/`): `catalog.json`, `events.jsonl`,
  `session-models.json`, `loop-events.jsonl`, optional weights

Overrides: `DOWNSHIFT_EVENT_LOG`, `DOWNSHIFT_SESSION_MODELS`, feature flags
documented in README and `docs/minilm-semantic.md`.

---

## Experimental systems (not production hook default)

| System | Role |
|--------|------|
| `routingv2` + shadow | Observe candidate classifier; does not change `Route()` |
| `decisionintelligence` | Advisory, monotonic, `Apply: false` |
| `orchestration/` (Python) | Opt-in LangGraph planner; does not pick models |
| Capability Router v2 doc | Long-term design; see [capability-router-v2.md](capability-router-v2.md) |

---

## Further reading

| Doc | Audience |
|-----|----------|
| [session-models.md](session-models.md) | Operators |
| [classifier-shadow.md](classifier-shadow.md) | Shadow mode |
| [benchmark/README.md](../benchmark/README.md) | Benchmark discipline |
| [design/router-generalization.md](design/router-generalization.md) | Evaluation honesty |
| [pt/README.md](pt/README.md) | Portuguese doc set |
