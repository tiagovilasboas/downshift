# Native quota routing evidence — 2026-10-09

harness-downshift by Tiago de Carvalho Vilas Boas  
https://github.com/tiagovilasboas/downshift

## Codex: live observation and local adapter replay

The development binary was run with an isolated temporary state directory and
`DOWNSHIFT_QUOTA_MODE=required`. Model availability came from the installed
Codex picker cache through `models discover --harness=codex`; quota came from
an actual local Codex transcript through `quota collect --harness codex`.
No account identifiers, transcript contents, or personal usage amounts are
included in this evidence document.

The replay used the controlled task `rename the variable userId`, a current
model present in the native picker, and the transcript path. The observed
subscription windows were fresh and available. The adapter emitted:

```json
{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"allow","updatedInput":{"message":"rename the variable userId","model":"gpt-6-luna","reasoning_effort":"low"}}}
```

Local telemetry recorded `outcome: rewrite_emitted`, `quota_status: available`,
and a change from `gpt-6-sol` to `gpt-6-luna`.

## Exhaustion control — simulated

For a separate replay, the same observation was supplied with both consumption
windows changed to 100 percent. This is a test input, not the account's live
consumption. The adapter returned allow without `updatedInput`. Telemetry
recorded `quota_status: exhausted` and retained `gpt-6-sol`.

## Limits of this evidence

This proves native availability/consumption ingestion and the local adapter's
quota-dependent rewrite decision. It does not prove that the running Codex
consumer supplies `transcript_path`, that the installed hook binary has been
updated, or that an executor honored the rewrite. No executor acknowledgement
is claimed. Other harnesses require their own source and runtime evidence.
