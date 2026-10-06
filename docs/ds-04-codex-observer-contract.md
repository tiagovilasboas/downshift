# DS-04: Codex observer lifecycle — privacy and evidence contract

Status: **POC contract only. Disabled by default.** This document does not
authorize a background process, hook/configuration change, remote API call, or
collection of Codex data.

## Evidence boundary

An observer may state only `evidence_state: "observed"`: a local lifecycle
signal was seen. It must never emit `executor_acknowledged`, `model_applied`,
or any equivalent claim. A lifecycle observation is not an acknowledgement from
Codex that a requested model, reasoning effort, tool call, or child action was
accepted or completed as intended.

The persisted event schema is exactly:

```json
{
  "schema_version": "harness-downshift.observer.v1",
  "evidence_state": "observed",
  "observed_at": "RFC3339 UTC timestamp",
  "lifecycle": "completed|needs_attention|failed",
  "observer_id_hash": "HMAC-SHA-256 hex",
  "subject_id_hash": "HMAC-SHA-256 hex",
  "association_state": "matched|ambiguous|unavailable",
  "correlation_id": "optional opaque lower-case hex ID"
}
```

No extra field is accepted. In particular, do not collect or persist
`prompt`, `messages`, `transcript`, `last_message`, `summary`, `title`, tool
arguments, source file contents, raw thread/session/agent IDs, account IDs,
model names, reasoning effort, or free-form errors. Errors are represented only
by the lifecycle enum. Hashes are identifiers for local aggregation, not proof
of identity or executor acceptance.

## Identity and retention

Raw observer, thread, session, or child IDs are normalized only in memory using
HMAC-SHA-256 with a per-host random key. The key is created only when the POC is
explicitly enabled, stored owner-only (`0600`), never exported, and rotated by
deleting prior observations. A fixed hash prefix or unsalted digest is not
acceptable.

The local JSONL target must be owner-only (`0700` directory, `0600` file), cap
retention at 1,000 events or 7 days, and prune before append. The observer must
not send this data to a model, provider, analytics service, or any MCP tool.

## Disabled-by-default operation

The only default is `enabled: false`: no polling, callback, observer process,
or hook registration. Enabling requires an explicit user request naming the
local POC, a visible configuration diff, and a fresh capability check of the
vendor-supported lifecycle surface. A missing, changed, or ambiguous vendor
contract keeps the observer disabled.

## Concurrency and correlation

Correlation is valid only when the observer receives the exact opaque ID from a
supported lifecycle contract. Timestamp, title, model, ordering, session, or
task similarity must never be used to infer it. For concurrent children where
the mapping is unavailable, persist `association_state: "ambiguous"` or
`"unavailable"` and omit `correlation_id`. Such an event is usable only as a
counted observation, not as routing evidence.

## Criteria before enabling the POC

1. A vendor-supported, documented lifecycle surface exposes the minimum
   metadata without transcript access.
2. The fixture sensor passes: strict field allowlist, forbidden-field rejection,
   opaque correlation format, and ambiguous-concurrency omission.
3. A local dry run proves permissions and retention without real conversation
   text, and verifies `0600`/`0700` ownership.
4. AppSec reviews the final adapter diff; the outer-harness CI executes the
   sensor. No provider ACK claim is added without a separately supported ACK
   contract.

## Fowler loop

| Concern | Guia | Sensor | Classification |
| --- | --- | --- | --- |
| Evidence truthfulness | `observed`-only state and ACK prohibition | Schema fixture rejects ACK/model claims | Guia computacional, behaviour; sensor computacional, behaviour |
| Privacy | Allowlist, HMAC and retention contract | Fixture rejects transcript/free-text/raw-ID fields | Guia computacional, behaviour; sensor computacional, behaviour |
| Concurrency | Explicit ambiguous/unavailable state | Fixture rejects guessed correlation | Guia inferencial, architecture fitness; sensor computacional, behaviour |
| Future enablement | Disabled-by-default and capability criteria | AppSec/vendor-contract review before adapter | Guia inferencial, architecture fitness; sensor inferencial, architecture fitness |

## Claude Code lifecycle adapter (Sep 2026)

`internal/adapters/claudecode/lifecycleadapter.go` maps Claude Code's
`SubagentStart` and `SubagentStop` hook payloads to `lifecycleobserver.Event`.

### Documented source fields (Claude Code hooks reference, Sep 2026)

| Hook | Fields decoded |
| --- | --- |
| `SubagentStart` | `hook_event_name`, `session_id`, `turn_id`, `agent_id`, `agent_type` |
| `SubagentStop`  | `hook_event_name`, `session_id`, `turn_id`, `agent_id`, `agent_type` |

No correlation ID is present in the Claude Code lifecycle schema as of Sep 2026.
The adapter therefore produces `association_state: "unavailable"` or
`"ambiguous"`, never `"matched"`. This is the same limitation as the Codex POC.

### Fields NOT decoded (explicitly absent from lifecycle contract)

`prompt`, `messages`, `transcript`, `model`, `reasoning_effort`, `tool_input`,
`correlation_id`. Extra fields in the JSON payload are silently ignored by
`encoding/json` (no `DisallowUnknownFields`), which is intentional.

### isSpawnTool update

`lifecycleobserver.isSpawnTool` was extended to recognise `"task"` (Claude Code)
alongside `"agent"` and `"spawn_agent"` (Codex). This lets the observer accept a
`PreToolUse{ToolName:"Task"}` fixture from a Claude Code session, which is
required for the three-event lifecycle (PreToolUse → SubagentStart → SubagentStop)
to reach `lifecycle: "completed"` in the integration test.

### Evidence boundary (unchanged)

`evidence_state: "observed"` only. Never `executor_acknowledged`, `model_applied`,
or any claim that the rewrite was accepted by the executor.
