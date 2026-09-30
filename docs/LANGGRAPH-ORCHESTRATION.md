# Local LangGraph orchestration

`harness-downshift` by Tiago de Carvalho Vilas Boas
https://github.com/tiagovilasboas/harness-downshift

## Boundary

`orchestration/` is an opt-in Python package that uses LangGraph only to validate and plan a bounded delegation cycle. It is local-only: it makes no LLM, tool, network, credential, or subprocess call, and it never spawns an agent.

Its direct dependency is exactly pinned in `pyproject.toml`; `uv.lock` resolves the full transitive set and CI uses `uv run --locked` so dependency drift fails the build rather than changing the planner silently.

The existing Go hook path remains the sole authority for task classification, tier selection, model choice, and hook rewrites.

Install it deliberately, separate from the zero-runtime Go binary:

```bash
python3 -m pip install -e orchestration
printf '%s' '{"source":"cli","task":"review a small change","delegate_tasks":["inspect code","run tests"],"max_delegates":2}' | python3 -m harness_downshift_orchestration
```

The CLI accepts exactly one JSON object on stdin (at most 1 MiB) and emits one JSON object on stdout. `status: planned` is advisory only: callers may submit each returned task to their harness, where Downshift's Go policy independently decides whether and how its model input can be rewritten. Rejected valid JSON requests return a JSON response with `status: rejected`; malformed input exits with code 2.

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
