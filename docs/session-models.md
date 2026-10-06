# Session models

Downshift may write only a model id that exists in the current harness session. The session list is the complete candidate set for every harness; the catalog is optional metadata (tier, cost, family, effort), never a source of candidate IDs.

Order every list from **least to most capable**. This ordering is the model-agnostic ranking contract and must reflect the actual picker choices for that session. Downshift maps abstract task tiers onto list positions (first for small, midpoint for mid, last for frontier), skips exhausted or catalog-known `explicit_only` entries, and never parses model names or relies on prices to pick a target. For downshifts, an `included` candidate within the eligible range may be preferred; upshifts always choose the strongest eligible session model. A single listed model can serve every tier. If capability ordering is unknown, correct the list rather than relying on the catalog to infer it.

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

`~/.harness-downshift/events.jsonl` stores routing outcomes: `complexity`, `estimated_savings`, `requested_model`, `final_model`, `harness`, optional hashed `session_id`, `timestamp`, `verdict`, and `outcome`, plus optional real-cost fields `input_tokens`, `output_tokens`, `cached_tokens`, `actual_cost_usd` and `baseline_cost_usd` (absent until a PostToolUse hook supplies provider usage; see `internal/telemetry/cost.go`). A raw Codex session ID is validated and hashed before it can be recorded. `requested_model` is `unknown` unless the original hook value is catalog-allowlisted; it is never filled with a policy recommendation. `outcome: "rewrite_emitted"` proves only that Downshift emitted a compatible rewrite, not that a child executor honored it. On Claude Code the PostToolUse hook also appends an `outcome: "resolved"` record that links to the decision (`linked_decision`) and carries the model the harness reports it chose for the child (`tool_response.resolvedModel`), with `rewrite_honored` true or false when the catalog can compare it to the written model. `downshift stats` counts a matching record as honored; a mismatch stays visible in the log. `loop-events.jsonl` stores classifier feedback metadata. Neither file is a copy of hook stdin and neither lists the models the picker offered.

Native config is not a session allowlist either:

- Claude Code `~/.claude/settings.json` holds hooks and a default model, not every model the account can select.
- Codex `config.toml` holds the active model and feature flags, not the session's selectable set.
- Cursor's model picker is UI state. This repo does not call a Cursor API to read it.

- Antigravity uses the `Subagents` array in `invoke_subagent` calls, where subagent models (`flash_lite`, `flash`, `pro`, `inherit`) are mapped directly according to routing decisions.

## Where the list comes from

Same order for every harness:

1. Hook payload, if it includes `session_models` or `available_models` (a JSON array of strings). A present field wins, including an empty array. The file is not read.
2. Otherwise `~/.harness-downshift/session-models.json` (override the path with `DOWNSHIFT_SESSION_MODELS`). An exact `sessions.<harness>.<session_id>` list wins when present; otherwise the top-level harness key (`cursor`, `claude-code`, or `codex`) is used as a compatibility fallback. A missing file or missing key means the session is unknown.

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

- Downshift or unknown verdict: choose the least capable, non-exhausted session ID at or above the abstract task tier's list-position threshold.
- Upshift: choose the strongest non-exhausted session ID.
- Guardrail hold (`Checked` and `SafeVerdict == OK`): do not rewrite.
- Unknown session: no rewrite.
- Any exact session ID can be written, even when the catalog does not know it. Sentinels such as `inherit`, exhausted IDs, and catalog-known `explicit_only` IDs cannot be selected.
- `inherit` may sit in the session list so the payload is recognised. It is never selected as the target.
- Optional `quota.<harness>.included` lists ids that still have token budget. When one of them shares the target tier, it wins over a metered id.
- Optional `quota.<harness>.exhausted` lists ids with no remaining budget. They are never selected. If the current id is exhausted, the hook moves to another session id that can still run. A hook payload may send `included_models` or `unavailable_models` and those arrays replace the file for that call.

For example, a list `["session-small", "session-mid", "session-frontier"]` makes those three positions available to all routing decisions without requiring any of the IDs or their prices in Downshift's catalog. If the session list is missing, the active model stays.
