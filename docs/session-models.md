# Session models

Downshift may write only a model id that exists in the current harness session. The session list is the complete candidate set for every harness; the catalog is optional metadata (tier, cost, family, effort), never a source of candidate IDs.

Order every list from **least to most capable**. This ordering is the model-agnostic ranking contract and must reflect the actual picker choices for that session. Upshift picks the last eligible id, so leaving an expensive frontier model off the list is the install-time opt-out. Claude Code accepts `fable`; the default example omits `claude-fable-5-1` so that upshift stops at Opus unless you append it. That Claude Code id is opt-in by listing it. It is not `explicit_only`. Downshift maps abstract task tiers onto list positions (first for small, midpoint for mid, last for frontier) inside the filtered session order, skips exhausted ids, and never parses model names or relies on prices to pick a target. A non-empty credit set for this harness is closed: ranking stays inside it, in session order, with no fallback to an id outside the set. Upshift picks the last remaining id. Catalog `explicit_only` ids are skipped unless `explicit_upshift` is on, and then only for upshift. A single listed model can serve every tier. If capability ordering is unknown, correct the list rather than relying on the catalog to infer it.

If the session list cannot be determined, the hook does not rewrite the model.

## Payload without a model field

Claude Code often spawns a Task without `tool_input.model`; the child then inherits the harness default (usually the parent's model). The router has no current model to compare against (`verdict: UNKNOWN`), so it applies guardrail **R6** (`R6_UNCONFIDENT_UNKNOWN_SMALL`):

- confident classification → the recommended model is written (any tier);
- unconfident classification with a **mid or frontier** recommendation → written (never below mid);
- unconfident classification with a **small** recommendation → not written; the event records `safe_verdict: OK` and `corrections: ["R6_UNCONFIDENT_UNKNOWN_SMALL"]`.

Uncertain calls therefore never move a spawn to the small tier, with or without a current model.

## What the hook actually sends

Checked against the adapter structs and local telemetry on 2026-09-27. No secrets below.

| Harness | Stdin fields the adapter decodes | Allowlist on the payload? |
|---|---|---|
| Cursor | `hook_event_name`, `tool_name`, `tool_input`, `model`, `model_id` | No. Current model only. |
| Claude Code | `hook_event_name`, `tool_name`, `tool_input`, `model`, `prompt` | No. Current model only. |
| Codex | `hook_event_name`, `session_id`, `tool_name`, `tool_input`, `model` | No. Current model and session id only. |

`~/.harness-downshift/events.jsonl` stores routing outcomes: `complexity`, `estimated_savings`, `requested_model`, `final_model`, `harness`, optional hashed `session_id`, `timestamp`, `verdict`, and `outcome`, plus optional real-cost fields `input_tokens`, `output_tokens`, `cached_tokens`, `actual_cost_usd` and `baseline_cost_usd` (absent until a PostToolUse hook supplies provider usage; see `internal/telemetry/cost.go`). A raw Codex session ID is validated and hashed before it can be recorded. `requested_model` is `unknown` unless the original hook value is catalog-allowlisted; it is never filled with a policy recommendation. `outcome: "rewrite_emitted"` proves only that Downshift emitted a compatible rewrite, not that a child executor honored it. On Claude Code the PostToolUse hook also appends an `outcome: "resolved"` record that links to the decision (`linked_decision`) and carries the model the harness reports it chose for the child (`tool_response.resolvedModel`), with `rewrite_honored` true or false when the catalog can compare it to the written model. `downshift stats` counts a matching record as honored; a mismatch stays visible in the log.

## Claude Code real usage

The PostToolUse payload of an async subagent is launch metadata with no tokens. Real usage is read when the subagent stops, from its own transcript (`agent_transcript_path`), by a second hook next to the PreToolUse one:

```json
"SubagentStop": [
  { "hooks": [ { "type": "command", "command": "downshift claude-code-subagent-stop" } ] }
]
```

It appends one `outcome: "usage"` record per agent: tokens (the last line per message id, since streaming repeats it), the model the API reported, and real cost. The hashed agent id (`agent_hash`) ties it to the launch-time `resolved` record and, through it, to the routing decision. Savings are real only when the decision knows the model the spawn would otherwise have used; when the original model was `unknown` the spawn is priced as unrouted and saves nothing. It never blocks and skips an agent it has already priced. `loop-events.jsonl` stores classifier feedback metadata. Neither file is a copy of hook stdin and neither lists the models the picker offered.

Native config is not a session allowlist either:

- Claude Code `~/.claude/settings.json` holds hooks and a default model, not every model the account can select.
- Codex `config.toml` holds the active model and feature flags, not the session's selectable set.
- Cursor's model picker is UI state. This repo does not call a Cursor API to read it.

- Antigravity uses the `Subagents` array in `invoke_subagent` calls, where subagent models (`flash_lite`, `flash`, `pro`, `inherit`) are mapped directly according to routing decisions.

## Where the list comes from

Same order for every harness:

1. Hook payload, if it includes `session_models` or `available_models` (a JSON array of strings). A present field wins, including an empty array.
2. Otherwise the models recovered for that harness by `downshift models discover` (`discovered.json`). See [session-discovery.md](session-discovery.md). A recovered list beats a handwritten allowlist.
3. Otherwise `~/.harness-downshift/session-models.json` (override the path with `DOWNSHIFT_SESSION_MODELS`). An exact `sessions.<harness>.<session_id>` list wins when present; otherwise the top-level harness key is the fallback. This file is not a credit report. `quota` in it is ignored.

The file is operator-curated. For Codex, `model` identifies the active model in that event and `session_id` identifies its session. Downshift does not call a picker API, discover account entitlements, or assume every catalog entry is selectable. Verify the session's actual choices before adding them to the allowlist.

Example with a per-session override (replace the session ID with the value in Codex hook input):

```json
{
  "codex": ["gpt-6-luna", "gpt-6-sol"],
  "sessions": {
    "codex": {
      "replace-with-session_id": ["gpt-6-luna", "gpt-6-sol"]
    }
  }
}
```

There is no built-in default list. `docs/examples/session-models.example.json` records one Cursor session from 2026-10-04. Copy it and edit it. Do not expect the binary to load that example on its own.

## How a target is chosen

The session list alone determines candidates and their relative capability. Catalog lookup may enrich a known ID with metadata or preserve `routing: explicit_only`; unknown IDs are not filtered, ranked by guessed names, or replaced.

- Downshift or unknown verdict: choose the list position for the abstract tier (first, midpoint, or last) on the filtered session order.
- Upshift: choose the last remaining id in that same filtered order.
- Guardrail hold (`Checked` and `SafeVerdict == OK`): do not rewrite.
- Unknown session: no rewrite.
- Any exact session ID inside the filtered set can be written, even when the catalog does not know it. `inherit` and exhausted IDs cannot be selected. A catalog `explicit_only` ID can be written only when `explicit_upshift` is on.
- `inherit` may sit in the session list so the payload is recognised. It is never selected as the target.
- Credits come only from this call's `included_models` and `unavailable_models`. There is no compiled-in credit list, and `quota` in `session-models.json` is not read.
- An exhausted id from that hook report is never selected, on any harness.
- When the hook reports `included_models`, downshift and upshift rank only inside that set, in session order, for that harness alone. There is no fallback to a session id outside the set. An empty intersection does not rewrite.
- When the hook did not report credits, ranking stays the session order minus exhausted ids, `inherit`, and `explicit_only` (unless explicit upshift is on). Downshift does not invent a credit denial.

## explicit_upshift

Default off. Without it, `explicit_only` models do not participate in upshift. With it, upshift (`VerdictUpshift` only) may select them. The last remaining id in the filtered session order wins. Downshift and ordinary tier mapping still skip `explicit_only`. If the current model is already `explicit_only`, it stays; the flag does not replace it.

An `explicit_only` id is selectable for upshift only when the flag is on and, if a credit set exists, the id is inside that set. `CanWriteSessionID` refuses the write otherwise, so the hook does not emit the upshift.

Turn the flag on with any of:

- `"explicit_upshift": true` in `session-models.json`
- `DOWNSHIFT_EXPLICIT_UPSHIFT=1`
- `install.sh --explicit-upshift` (the version argument still works before or after the flag)

Cursor `claude-fable-5-1-thinking-high` and Codex `gpt-6-astra` are `explicit_only`. Claude Code `claude-fable-5-1` is not: list that id in the session when you want upshift to reach it. Do not alias another harness's canonical id.

For example, a list `["session-small", "session-mid", "session-frontier"]` makes those three positions available to all routing decisions without requiring any of the IDs or their prices in Downshift's catalog. If the session list is missing, the active model stays.
