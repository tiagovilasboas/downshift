# Cursor live routing (2026-10-24)

Downshift by Tiago Vilas Boas
https://github.com/tiagovilasboas/downshift

One sentence: a Cursor Task spawn already reaches `downshift cursor`; the
shared runner held it because the current model arrived empty and the read
was unconfident, and the end hook left no record. The spike is
[spike.md](spike.md). This batch is the Cursor slice of A-P5 in
[the internal roadmap](../../roadmap.md).

Doubt stays allow. R7, R8, and R9 are not edited. The token account
(`telemetry.Summarize`) and the dollar account (`accounting`, ADR Proposed)
are not this batch. No task merges them.

## Coverage inventory

Filled from [architecture.md](../../../design/architecture.md),
[domains/README.md](../../../domains/README.md), and
[capability-matrix.md](../../../reference/capability-matrix.md).
T3 and T4 stay blocked while their effect cell is `unknown`.

| Surface | Packages | Effect | Account | This batch |
|---|---|---|---|---|
| Routing | `core`, `classify`, `fuse`, `session`, `catalog`, `quota` | writes the child model | `telemetry.Summarize` | out |
| Shell | `execwrap`, `shellwrap` | rewrites a command | bytes, outside `Saved` | out |
| DCI | `codeintel` | injects context | latency and bytes | out |
| Telemetry | `telemetry`, `childend`, `hookport` | measures | token, no dollar | out |
| Accounting | `accounting` | CLI does not import | dollar, +15% bias | out |
| Adapters | `internal/adapters/cursor` | translates the wire | honor and usage for Cursor | in, after T2 |
| Quality | `outcome`, `benchmark`, `runHookAdapter` / `runEnd` tests | offline | acceptance | in, on T3 and T4 |

`cmd/downshift` is in only for the probe wiring task (T2) and the existing
`runEnd` test. Setup's hook list does not gain `subagentStart`.

## Tasks

Estimates are story points (complexity and uncertainty). Ceiling is 5. Each
task is one PR.

| # | Pts | Task | Depends on | Closes when |
|---|-----|------|------------|-------------|
| T1 | 2 | Spike: one live Task spawn, harness counts, and the layer the hold belongs to | — | [spike.md](spike.md) names the row, the rule, and the two UNKNOWNs. Done 2026-10-24 |
| T2 | 2 | Probe field names. Add `downshift probe preToolUse` and `downshift probe subagentStop` next to the existing Cursor hooks, spawn one mechanical task, copy the probe lines (model ids and field names only) into the spike, then remove the probe hooks | T1 | spike.md replaces both UNKNOWN rows with the fields that were present, or with "key absent". **Done 2026-10-24** |
| T3 | 3 | If T2 shows a parent-model field the adapter does not read, read it in `RequestedModel` and pass it to `core.Route`. Redacted fixture, test through `runHookAdapter` | T2, and the field exists | **Closed without adapter edit:** preToolUse on 3.27.12 has no model field; matrix updated |
| T4 | 3 | If T2 shows `subagentStop` ran, map the real payload in `ParseChildEnd` and test through `runEnd`. The executed model is recorded only when a field actually carries it | T2, and the event ran | **Done:** `SessionIdentifier` + `subagent_id` link; `TestEndCursorSubagentStopLinksBySubagentIDWhenToolCallAbsent` |
| T5 | 1 | Write the 2026-10-24 result into `docs/reference/harness-matrix.md` and `docs/reference/capability-matrix.md`: hook fires on 3.27.12; honor stays unconfirmed until a rewrite and a child model are both observed | T3–T7 | **Done 2026-10-24** (harness-matrix + capability-matrix) |
| T6 | 3 | Transcript baseline on `preToolUse` when `model_id` is absent (`internal/adapters/cursor/sessionmodel.go`) | T2 | `TestHandle_TranscriptBaselineTrivialDownshift` + matrix bullet on transcript_path. **Done**; native JSONL has no `model` field, so live baseline still needs T7 |
| T7 | 3 | Composer slug cache: `cursor-touch` on `sessionStart` / `beforeSubmitPrompt`, `TouchFromStop` on `subagentStop`, `~/.downshift/cursor-composer-model.json` | T2 | `TestComposerCache_RecallAfterTouch`, `setup` wires `cursor-touch`. **Done 2026-10-24** |

## Order

```
T1 (done)
  └─ T2 probe
       ├─ T3 adapter model field     parallel, file: cursor.go RequestedModel path
       └─ T4 adapter end parser      parallel, file: childend.go
            └─ T5 matrix
```

T3 and T4 share no file. They start only after T2 names the fields. A task
whose probe answer is "absent" does not become a code change.

## Out of this batch

- Public copy (install, README, site): [2026-10-24-cursor-docs-site](../2026-10-24-cursor-docs-site/README.md).
- harness-core `gen-cursor-rules.sh --apply` (the always-on compactor rule is stale; that is the rules repo).
- Rebuilding the Downshift binary for the two commits after `480391dc` (changelog and `gh run view --log`).
- Native Cursor quota export (built-in-only API, [diagnostic](../../../evidence/cursor-gap-diagnostic.md)).
- Wiring `subagentStart` as a router. The hooks page gives it no `updated_input`.
- A rewrite-honored claim. Cursor's child transcript still has no executor model on the 2026-10-10 capture.
- Codex A-P5. Separate payload, separate task, when this Cursor slice is closed.

## Guia and sensor

| Concern | Guia | Sensor | Eixo |
|---|---|---|---|
| One spawn reaches the Cursor hook | spike table | the `cursor` / `allow` row at `2026-10-24T14:48:35Z` | behaviour |
| Empty model is parsed at the edge | layering checklist | T3 test through `runHookAdapter`, or a matrix row if no field exists | architecture fitness |
| End evidence is a real payload | `ParseChildEnd` comment | T2 probe line and, if it ran, a `runEnd` test | behaviour |
| Doubt stays allow | R9 in `core.Check` | the spawn's `corrections` stayed `R9_UNCONFIDENT_UNKNOWN_MID` | behaviour |
