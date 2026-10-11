# Agent reference (on demand)

Detail moved out of `AGENTS.md` so every agent session loads fewer tokens. Read the
section you need; the mandatory rules stay in `AGENTS.md`.

## Context compressor

Routing is one token lever. The other is the native compressor
(`downshift context compress`, `internal/compressor`): it compacts an all-`ok`
`go test`, a clean `git status`, and repetitive logs only when safe mode is
given exit code 0. It preserves non-`ok` lines, search results, file lists,
git logs, and any non-zero exit. `context benchmark` does not measure
routing cost or accuracy. The compressor table is bytes from checked-in
fixtures, not tokens and not dollars.
Read [docs/context-optimization.md](../context-optimization.md) before
changing context handling or attributing savings to model routing alone.

## Telemetry and cost reporting

Downshift records local routing decisions to `events.jsonl` in the state directory (`~/.downshift/events.jsonl`). Tasks, prompts, and file paths are never stored.

Commands for agents and operators to audit savings, costs, and optimization:
- `downshift stats`: display decision counts and estimated token savings (default: 30 days).
- `downshift stats --days=N`: filter metrics window to the last N days.
- `downshift stats --cost-per-unit=<USD>`: convert normalised unit savings to estimated USD using list prices.
- `downshift stats --export`: generate sanitized, prompt-free JSON summary for audits and issues. See [docs/stats-export.md](../stats-export.md).
- `downshift serve`: start local web dashboard at `http://localhost:7474` (serves `web/`).
- `dsmon`: standalone terminal dashboard widget (`cmd/dsmon/launch.sh`).

---

## catalog.json — embedded vs user override

**Two distinct files:**

1. `internal/catalog/catalog.json` — the **embedded default**, committed to
   the repository and compiled into the binary via `go:embed`. This IS in the
   repo and IS versioned. Edit it to update model data for all users.

2. `~/.downshift/catalog.json` (legacy: `~/.harness-downshift/`) — the **user override**, personal data
   that lives outside the repo. This is excluded by `.gitignore`. The user
   creates it via `downshift models pull` or by copying `catalog.sample.json`.

When the agent says "do not commit catalog.json", it means the **user override**
(under the state dir; see `docs/config.md`), not the embedded default.

**Adding or updating a model:** edit `internal/catalog/catalog.json`. The
`_section` comment objects (entries with only a `_section` key, no `id` or
`harness`) are silently skipped by the parser — use them for readability.
Every real entry must have `id`, `harness`, `tier`, and `family`.

---

## Post-spawn honor and usage seam

`downshift honor <harness>` and `downshift usage <harness>` are the generic entry points for the
two post-spawn observations (`internal/hookport`). They are not in `downshift help` because users
run the Claude Code names instead: `claude-code-post-tool-use` (records the model the child ran,
`resolved`) and `claude-code-subagent-stop` (child token usage and cost). Both Claude Code commands
call the same runners.

Only a harness whose own hook sends the field is registered in `hookPorts` (`cmd/downshift/main.go`).
Every other harness gets a port with nil functions, so the stage stays "unobserved" instead of
borrowing another harness's payload. To add a harness: implement its `hookport.Port`, register it,
then document its hook command. These observations feed the honest telemetry and real-cost pillars
in `docs/beta-exit.md` (P1, P3), so keep the seam.

## Child end status and baseline source (E8, E7)

- `stats` prints "How children ended" and the export carries `child_status`, both from `Aggregate` (shared layer, only when non-zero).
- `child_ended` (`internal/telemetry/child_outcome.go`): a closed `ChildStatus` from
  `ClassifyChildEnd(event, tool_response.status, is_interrupt)`. The error text is
  never read. Recorded by `recordChildEnd` in the Claude Code adapter for Agent/Task
  calls only. It is in `IsCostOnlyOutcome` and skipped in `Aggregate` before the
  decision count, so it can never move `total`.
- `baseline_source`: set to `requested` wherever a baseline cost is computed. The
  constant `telemetry.BaselineRequested` is the only producer. `inferred` and
  `declared` are not produced; adding one needs a reviewed design, and `inferred`
  must stay out of headline totals by default.
