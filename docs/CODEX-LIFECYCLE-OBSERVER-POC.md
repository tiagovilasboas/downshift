# Codex lifecycle observer POC (DS-04)

harness-downshift by Tiago de Carvalho Vilas Boas  
https://github.com/tiagovilasboas/downshift

This is a local, disabled-by-default POC at
`internal/lifecycleobserver/`. It accepts only in-memory **synthetic fixtures**
of Codex lifecycle events. It has no command, hook registration, stdin reader,
transcript access, persistence, provider SDK, or network client. It therefore
does not alter `~/.codex/hooks.json`, `config.toml`, or any user configuration.

## Contract and evidence boundary

The fixture contract uses the documented Codex hook fields:

- `PreToolUse`: `session_id`, `turn_id`, `tool_name` (Codex `Agent` or a
  namespaced `spawn_agent` tool);
- `SubagentStart`: `session_id`, `turn_id`, `agent_id`, `agent_type`;
- `SubagentStop`: `session_id`, `turn_id`, `agent_id`, `agent_type`.

The observer maps a single `PreToolUse` to a single `SubagentStart` by
`session_id + turn_id`, then maps the start to a stop by that pair plus
`agent_id + agent_type`, only in memory. Its versioned output has no raw ID:
`observer_id_hash` and `subject_id_hash` are HMAC-SHA-256 values under
caller-supplied ephemeral key material; `observed_at` is UTC. The output always
has `evidence_state: "observed"` and reports an `association_state`:

| State | Meaning |
| --- | --- |
| `unavailable` | A required fixture or a documented opaque correlation field is absent. |
| `ambiguous` | Multiple candidates share the lifecycle key; no match is guessed. |
| `matched` | Reserved for a future vendor contract that supplies an exact opaque correlation ID. |

The current Codex lifecycle fields contain no documented opaque routing
correlation ID. Therefore this POC emits `unavailable` for even a complete
lifecycle and omits `correlation_id`; it emits `ambiguous` for concurrent
candidates and omits it there too. `matched` is not produced by the POC.

`observed` is lifecycle evidence only. It never means
`executor_acknowledged`, does not inspect the effective model, and does not
upgrade DS-01's `rewrite_emitted` evidence.

The documented lifecycle schema says `SubagentStart` and `SubagentStop` expose
the session/turn/agent fields used here, while transcript paths are not a stable
hook API. This POC deliberately excludes those paths and all message content.
See [Codex Hooks documentation](https://developers.openai.com/docs/hooks).

## Local test

```bash
go test -race ./internal/lifecycleobserver
go vet ./...
```

## Fowler loop

| Concern | Guia | Sensor | Classification |
| --- | --- | --- | --- |
| Correlation correctness | Narrow, documented fixture schema and unavailable/ambiguous fail-closed contract | Synthetic full, missing and concurrent lifecycle tests | Guia computacional, architecture fitness; sensor computacional, behaviour |
| Privacy | No transcript/message fields, I/O or persistence; HMAC-only output | Anti-leak/schema test and package-only test execution | Guia computacional, behaviour; sensor computacional, maintainability |
| Evidence semantics | `observed` vocabulary excludes executor acknowledgement | Test requires only lifecycle states and docs state the limit | Guia inferencial, architecture fitness; sensor computacional, behaviour |
| Future native integration | Disabled-by-default constructor; no hook/config integration | AppSec/AI-FinOps review before any runtime adapter | Guia inferencial, architecture fitness; sensor inferencial, architecture fitness |
