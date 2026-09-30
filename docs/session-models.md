# Session models

Downshift may write only a model id that exists in the current harness session. The catalog is metadata (tier, cost, family, effort) for an id that is also in that session. It is not the candidate set.

If the session list cannot be determined, the hook does not rewrite the model.

## What the hook actually sends

Checked against the adapter structs and local telemetry on 2026-09-27. No secrets below.

| Harness | Stdin fields the adapter decodes | Allowlist on the payload? |
|---|---|---|
| Cursor | `hook_event_name`, `tool_name`, `tool_input`, `model`, `model_id` | No. Current model only. |
| Claude Code | `hook_event_name`, `tool_name`, `tool_input`, `model`, `prompt` | No. Current model only. |
| Codex | `hook_event_name`, `session_id`, `tool_name`, `tool_input`, `model` | No. Current model and session id only. |

`~/.harness-downshift/events.jsonl` stores routing outcomes: `complexity`, `estimated_savings`, `requested_model`, `final_model`, `harness`, optional hashed `session_id`, `timestamp`, `verdict`, and `outcome`. A raw Codex session ID is validated and hashed before it can be recorded. `requested_model` is `unknown` unless the original hook value is catalog-allowlisted; it is never filled with a policy recommendation. `outcome: "rewrite_emitted"` proves only that Downshift emitted a compatible rewrite, not that a child executor honored it. `loop-events.jsonl` stores classifier feedback metadata. Neither file is a copy of hook stdin and neither lists the models the picker offered.

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

There is no built-in default list. `docs/examples/session-models.example.json` records one Cursor session from 2026-09-27. Copy it and edit it. Do not expect the binary to load that example on its own.

## How a target is chosen

Candidates are the session ids. For each id, catalog lookup adds tier and cost when the id (or its alias) belongs to that harness. Ids the catalog does not know stay eligible. They are not replaced by a catalog id.

- If the classified catalog model is in the session, that session string is the target.
- If it is not, a downgrade uses the cheapest catalog-labeled session id (lower tier, then lower input+output cost). An upshift uses the strongest labeled session id.
- If no session id is in the catalog, a downgrade uses the first id in the file. List cheapest models first. An upshift does not guess among unlabeled ids.
- `routing: explicit_only` is never an automatic target. If the current model is explicit-only, it stays.
- Unknown session: no rewrite.

So if the catalog smallest model is `claude-4.5-haiku-thinking` and the session only has `claude-4.5-sonnet-thinking` and a frontier id, the target is the sonnet id. If the session list is missing, the frontier model stays.
