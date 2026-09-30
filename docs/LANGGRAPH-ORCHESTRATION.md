# Orchestration planner — design notes

`harness-downshift` by Tiago de Carvalho Vilas Boas  
https://github.com/tiagovilasboas/harness-downshift

## Execution path: Go (`internal/orchestration/`)

Fan-out delegation planning lives natively in Go, in `internal/orchestration/planner.go`.
Zero Python, zero runtime overhead (~0ms). Same contract as the reference design below.

```go
plan := orchestration.Orchestrate(orchestration.Request{
    Source:        "hook",
    Task:          "review a small change",
    DelegateTasks: []string{"inspect code", "run tests"},
    MaxDelegates:  2,
})
// plan.Status == "planned"
// plan.Delegations == [{1, "inspect code"}, {2, "run tests"}]
```

## Reference design: Python / LangGraph (`orchestration/`)

The `orchestration/` Python package is kept as a **reference artifact and training baseline**.
It implements the identical contract using LangGraph StateGraph, which makes it
useful for visualising the decision graph and experimenting with future extensions
(e.g. weighted ordering, dataset-driven policy). It is **not** the execution path.

Its direct dependency is exactly pinned in `pyproject.toml`; `uv.lock` resolves the
full transitive set and CI uses `uv run --locked`.

Run it standalone for debugging or comparison:

```bash
printf '%s' '{"source":"cli","task":"review a small change","delegate_tasks":["inspect code","run tests"],"max_delegates":2}' \
  | python3 -m harness_downshift_orchestration
```

## Contract v1

Request:

```json
{
  "source": "hook",
  "correlation_id": "0123456789abcdef0123456789abcdef",
  "task": "parent task summary",
  "delegate_tasks": ["bounded child task"],
  "max_delegates": 1,
  "capabilities": {"max_parallel_delegates": 1}
}
```

`source` is an explicit transport enum: `cli`, `hook`, or `service`; it deliberately does not identify a vendor, model, or agent. `correlation_id` is optional but, when supplied, must be 16-64 lowercase hexadecimal characters and is preserved in the response. `task` is a non-empty string. `delegate_tasks` is a list of non-empty strings. `max_delegates` is an integer from 0 to 8. Unknown request/capability fields are rejected. The graph removes duplicate tasks while preserving first-seen order, then returns at most that many `delegations` with a one-based ordinal. It does not infer subtasks or choose an agent/model; that requires an explicit caller policy.

`capabilities.max_parallel_delegates` is an optional, vendor/model/agent-agnostic caller contract (also 0 to 8). The effective bound is the lower of that capability and `max_delegates`; absence means the local maximum of 8. No provider SDK type or model identifier crosses this boundary.

Response fields include the stable `schema_version`, `status`, `delegations`, an auditable node `trace`, and `routing_boundary` to prevent a consumer from mistaking planning for tier/model routing.

The CLI fails closed after two seconds waiting for stdin and emits `INPUT_TIMEOUT`. It appends only timestamp, status, error code, and delegation count to `~/.harness-downshift/orchestration-events.jsonl` with owner-only permissions; task text and prompt content are never logged. Set `DOWNSHIFT_ORCHESTRATION_TELEMETRY_PATH` only for controlled test environments.

## Fowler loop

| Concern | Guia | Sensor | Classification |
| --- | --- | --- | --- |
| Keep orchestration separate from routing | This boundary and the typed JSON v1 contract | Unit tests assert planning/rejection; response labels the routing boundary | Guia inferencial, architecture fitness; sensor computacional, architecture fitness |
| Bound delegation safely | Strict schema/enum validation, 1 MiB input limit, two-second deadline, 0-8 delegate limit | Graph tests cover invalid input, deduplication, and cap; prompt-free local telemetry signals operating outcomes | Guia computacional, behaviour; sensor computacional, behaviour |
| Preserve a usable planner | Local-only/opt-in usage and documented install command | CI installs the package and runs its unit tests | Guia inferencial, maintainability; sensor computacional, maintainability |

Continuous operational signals are intentionally deferred: the planner has no execution or completion event. If an executor is added, it must keep the opt-in/fail-open boundary and add prompt-free, local observability before it claims a closed loop.

## See also

For a complete, beginner-friendly explanation of how LangGraph fits here and
how LangChain fits in Harness Central — including what each framework does, what
it does not do, and how the two relate — see
[docs/LANGCHAIN-LANGGRAPH-EXPLAINER.md](https://github.com/tiagovilasboas/agent-harness/blob/main/docs/LANGCHAIN-LANGGRAPH-EXPLAINER.md)
in `harness-core`.
