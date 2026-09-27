# Session models

Downshift may write only a model id that exists in the current harness session. The catalog is metadata (tier, cost, family, effort) for an id that is also in that session. It is not the candidate set.

If the session list cannot be determined, the hook does not rewrite the model.

## What the hook actually sends

Checked against the adapter structs and local telemetry on 2026-09-27. No secrets below.

| Harness | Stdin fields the adapter decodes | Allowlist on the payload? |
|---|---|---|
| Cursor | `hook_event_name`, `tool_name`, `tool_input`, `model`, `model_id` | No. Current model only. |
| Claude Code | `hook_event_name`, `tool_name`, `tool_input`, `model`, `prompt` | No. Current model only. |
| Codex | `hook_event_name`, `tool_name`, `tool_input`, `model` | No. Current model only. |

`~/.harness-downshift/events.jsonl` (263 rows) stores routing outcomes: `complexity`, `estimated_savings`, `from`, `harness`, `timestamp`, `to`, `verdict`. `loop-events.jsonl` (195 rows) stores `confident`, `features`, `harness`, `id`, `record_type`, `selected_tier`, `timestamp`. Neither file is a copy of hook stdin and neither lists the models the picker offered.

Native config is not a session allowlist either:

- Claude Code `~/.claude/settings.json` holds hooks and a default model, not every model the account can select.
- Codex `config.toml` holds the active model and feature flags, not the session's selectable set.
- Cursor's model picker is UI state. This repo does not call a Cursor API to read it.

Antigravity is unfinished local work and is not part of this path.

## Where the list comes from

Same order for every harness:

1. Hook payload, if it includes `session_models` or `available_models` (a JSON array of strings). A present field wins, including an empty array. The file is not read.
2. Otherwise `~/.harness-downshift/session-models.json` (override the path with `DOWNSHIFT_SESSION_MODELS`). The harness key must be present (`cursor`, `claude-code`, or `codex`). A missing file or a missing key means the session is unknown.

There is no built-in default list. `docs/examples/session-models.example.json` records one Cursor session from 2026-09-27. Copy it and edit it. Do not expect the binary to load that example on its own.

## How a target is chosen

Candidates are the session ids. For each id, catalog lookup adds tier and cost when the id (or its alias) belongs to that harness. Ids the catalog does not know stay eligible. They are not replaced by a catalog id.

- If the classified catalog model is in the session, that session string is the target.
- If it is not, a downgrade uses the cheapest catalog-labeled session id (lower tier, then lower input+output cost). An upshift uses the strongest labeled session id.
- If no session id is in the catalog, a downgrade uses the first id in the file. List cheapest models first. An upshift does not guess among unlabeled ids.
- `routing: explicit_only` is never an automatic target. If the current model is explicit-only, it stays.
- Unknown session: no rewrite.

So if the catalog smallest model is `claude-4.5-haiku-thinking` and the session only has `claude-4.5-sonnet-thinking` and a frontier id, the target is the sonnet id. If the session list is missing, the frontier model stays.
