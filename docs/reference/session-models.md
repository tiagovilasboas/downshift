# Session models

Downshift may write only a model id that exists in the current harness session. The session list is the complete candidate set for every harness; the catalog never adds candidate IDs. Metadata is optional for capability-ordered hook/operator lists. An unordered native Cursor export requires resolver tier metadata before its exact IDs become routing targets.

Order every list from **least to most capable**. This ordering is the model-agnostic ranking contract and must reflect the actual picker choices for that session. A confident upshift picks the least eligible model at the tier the task needs (later list position first within a tier) and never goes below the current model's tier; when the baseline is guessed (Claude Code settings, Cursor parent `model_id`) or `explicit_upshift` is on, it keeps the strongest eligible tier; a list the catalog cannot rank falls back to the position mapping. Leaving an expensive frontier model off the list is the install-time opt-out. Claude Code accepts `fable`; the default example omits `claude-fable-5-1` so that upshift stops at Opus unless you append it. That Claude Code id is opt-in by listing it. It is not `explicit_only`. Downshift maps abstract task tiers onto list positions (first for small, midpoint for mid, last for frontier) inside the filtered session order, skips exhausted ids, and never parses model names. On the catalog-tier path prices do not pick a target; the optional capability path (below) uses catalog prices only to choose the cheapest candidate that already passes its score. A non-empty credit set for this harness is closed: ranking stays inside it, in session order, with no fallback to an id outside the set. When no tier is known, upshift picks the last remaining id. Catalog `explicit_only` ids are skipped unless `explicit_upshift` is on, and then only for upshift. A single listed model can serve every tier. If capability ordering is unknown, correct the list rather than relying on the catalog to infer it.

If the session list cannot be determined, the hook does not rewrite the model.

## Session metadata before catalog fallback

All five hook adapters resolve the session before classifying its current model.
Exact `model_metadata` descriptors take precedence over catalog capability and
effort data. The catalog supplies optional fallback metadata; it cannot add an ID
to the session. A declared least-to-most-capable `session_models` list can rank
otherwise unknown IDs. An unordered `available_models` inventory cannot.

For a harness that supplies descriptors, the shared payload extension is:

```json
{
  "session_models": ["session-fast", "session-strong"],
  "model_metadata": [
    {"id": "session-fast", "tier": "small", "efforts": ["low", "medium"]},
    {"id": "session-strong", "tier": "frontier", "efforts": ["low", "medium", "high"]}
  ]
}
```

`tier` is `small`, `mid` or `frontier`. Optional `input_per_million` and
`output_per_million` are independently supplied prices; omitted prices are
unknown. This extension is accepted by the adapters, not a claim that every
native harness emits it. Discovery preserves descriptors when its source exposes
them. Picker order, display names and reasoning-effort counts do not establish
capability. Missing evidence holds routing instead of assigning a model by name.

Guia: this shared session metadata contract (inferencial, architecture fitness).
Sensor: shared resolver, all-adapter acceptance tests and unknown-price regression
checks (computacional, architecture fitness / behaviour).

## Payload without a model field

Claude Code often spawns a Task without `tool_input.model`; the child then inherits the harness default (usually the parent's model). The router has no current model to compare against (`verdict: UNKNOWN`), so it applies guardrails **R6** (`R6_UNCONFIDENT_UNKNOWN_SMALL`), **R9** (`R9_UNCONFIDENT_UNKNOWN_MID`) and **R7** (`R7_UNCONFIDENT_UNKNOWN_FRONTIER`):

- confident classification → the recommended model is written (any tier);
- unconfident classification with a **mid** recommendation → not written (R9), unless a deterministic rule lifted it (risk floor, an opted-in NB upshift, an operator tier floor). A prompt with no recognised signal falls to MEDIUM by default, so writing mid over the harness default could move a task up for no reason;
- unconfident classification with a **small** recommendation → not written (R6); the event records `safe_verdict: OK` and `corrections: ["R6_UNCONFIDENT_UNKNOWN_SMALL"]`;
- unconfident classification with a **frontier** recommendation → not written (R7), unless a deterministic rule lifted it (risk floor, an opted-in NB upshift, an operator tier floor). The child would otherwise inherit the default, so writing the dearest tier on doubt is an unconfident upshift in disguise.

With a known current model, **R8** (`R8_UNCONFIDENT_UPSHIFT`) is the mirror of R1: an upshift on an unconfident classification is held. Uncertain calls therefore never spend money, or save it, in either direction, with or without a current model. The semantic boost cannot create a COMPLEX classification on its own: Complex needs regex evidence or an applicable deterministic safety lift. Graph escalation was removed in 2026-10-17.

### Baseline model when the call has no `model` (Claude Code)

For a Task call without `model` on an inheriting agent type (no `subagent_type`, `general-purpose`, `claude`, `Explore` or `Plan`), the model the child would run on is resolved in the order Claude Code uses: the per-call `model`, the payload `model`, then the variable named by `HarnessCapabilities.ModelOverrideEnv` (`CLAUDE_CODE_SUBAGENT_MODEL`) when set and not `inherit` (for `Explore` and `Plan` only with `CLAUDE_CODE_SUBAGENT_MODEL_FORCE` on), then the last assistant model in the parent transcript. When the transcript has no assistant message yet (a headless run, or parallel Agent calls in the first message), the configured model is the fallback (`ANTHROPIC_MODEL`, then the project and user `settings.json`; `opusplan` reads as Opus under plan mode, Sonnet otherwise) and the decision carries `BASELINE_FROM_SETTINGS`; a downshift on that guess goes only to the small tier (`HELD_GUESSED_BASELINE` when held). Verdict and savings are computed against that baseline. Other agent types may pin a model in their frontmatter, so they get no inferred baseline, and `Explore` under a Fable session gets none either. The full order, with code locations and the dated limits, is in [guides/claude-code.md](../guides/claude-code.md).

## What the hook actually sends

Checked against the adapter structs and local telemetry on 2026-10-17. No secrets below. Optional extension fields (`session_models`, `available_models`, `model_metadata`, `included_models`, `unavailable_models`, `usage_quota`) are decoded when present on any harness; none of the native PreToolUse payloads send a full picker list today.

| Harness | Stdin fields the adapter decodes (PreToolUse) | Allowlist on the payload? |
|---|---|---|
| Cursor | `hook_event_name`, `tool_name`, `tool_input`, `model`, `model_id`, optional `cwd`, `workspace_roots`, `tool_use_id`, `correlation_id` | No. Current model only unless extension fields are sent. |
| Claude Code | `hook_event_name`, `tool_name`, `tool_input`, `model`, `prompt`, optional `session_id`, `cwd`, `transcript_path`, `permission_mode`, `tool_use_id`, `correlation_id`, `parent_model`, `entry` | No. Current model only unless extension fields are sent. |
| Codex | `hook_event_name`, `session_id`, `tool_name`, `tool_input`, `model`, optional `cwd`, `transcript_path`, `tool_use_id`, `correlation_id` | No. Current model and session id only unless extension fields are sent. |

`~/.downshift/events.jsonl` (legacy `~/.harness-downshift/`) stores routing outcomes: `complexity`, `estimated_savings`, `requested_model`, `final_model`, `harness`, optional hashed `session_id`, `timestamp`, `verdict`, and `outcome`, plus optional real-cost fields `input_tokens`, `output_tokens`, `cached_tokens`, `actual_cost_usd` and `baseline_cost_usd` (absent until a harness reports usage — on Claude Code that is **SubagentStop** pricing the child transcript, not PostToolUse on an async Agent launch, which carries no tokens; see [beta-exit.md](beta-exit.md) P3 note and [Claude Code real usage](#claude-code-real-usage); `internal/telemetry/cost.go`). A raw Codex session ID is validated and hashed before it can be recorded. `requested_model` is `unknown` unless the original hook value is catalog-allowlisted; it is never filled with a policy recommendation. `outcome: "rewrite_emitted"` proves only that Downshift emitted a compatible rewrite, not that a child executor honored it. On Claude Code the PostToolUse hook also appends an `outcome: "resolved"` record that links to the decision (`linked_decision`) and carries the model the harness reports it chose for the child at launch (`tool_response.resolvedModel`), with `rewrite_honored` true or false when the catalog can compare it to the written model — still not token usage. `downshift stats` counts a matching record as honored; a mismatch stays visible in the log.

## Claude Code real usage

The PostToolUse payload of an async `Agent`/`Task` subagent is **launch** metadata only (`isAsync`, `resolvedModel`, …) with **no token usage** — the same fact recorded under P3 in [beta-exit.md](beta-exit.md). Real usage is read when the subagent stops, from its own transcript (`agent_transcript_path`), by a separate hook next to the PreToolUse one:

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

- Antigravity uses the `Subagents` array in `invoke_subagent` calls; routing inspects the last entry with a `Prompt` and rewrites that child's model (`flash_lite`, `flash`, `pro`, `inherit`).

## Main session policy: plan mode and opusplan (Claude Code)

Downshift routes subagents; the main session is the harness's. For Claude Code, `downshift setup` also sets three keys in `~/.claude/settings.json`, each only when it is not already set:

| Key | Value | Effect |
|---|---|---|
| `model` | `opusplan` | Opus while plan mode is on, Sonnet for execution ([model config](https://code.claude.com/docs/en/model-config)) |
| `permissions.defaultMode` | `plan` | Every terminal session starts in plan mode: Claude reads and plans, and edits stay blocked until you approve the plan ([permission modes](https://code.claude.com/docs/en/permission-modes)) |
| `enabledPlugins["cc-plugin-you-should-know@builtin"]` | `true` | Turns on Claude Code's built-in "You should know" mod (off by default, Claude Code 2.1.287+): a side agent rereads the conversation from time to time and shows one note above the prompt when it finds something you might have missed ([interactive mode](https://code.claude.com/docs/en/interactive-mode#you-should-know-suggestions)). It spends usage: each check is a billed background request to the session's model, and the mod's instructions and the reply are billed in full every time. `/plugin disable cc-plugin-you-should-know@builtin` turns it off. |

The check is on the sub-key, not on `enabledPlugins`: the map usually already holds other plugins (the Downshift plugin itself), and they are kept. A `false` you set is respected and reported by `doctor` as `you-should-know false (set by you)`. The settings key is the one `/plugin enable` is expected to persist; Claude Code's docs name the command, not the key, so confirm it on a live session. `setup` does not write `autoCompactWindow`: it is not part of the policy. Users who want auto-compact earlier on 1M-context models set it themselves (a plain integer such as `400000`; Claude Code caps it at the model window, so it is a no-op on 200k models, and `400k` would be read as 400). Per-subagent windows are a separate setting (`autoCompactWindow` in the agent's frontmatter, Claude Code 2.1.296+) and Downshift does not set them.

Together the three make the plan/execute split and the background checks happen in every session. A value you already set is never replaced; setup prints how to switch, and `downshift doctor` shows all three as `main session`. A new `defaultMode` replaces Claude Code's built-in starting mode (auto mode on recent versions), so a session now starts by planning instead of acting. Approving a plan leaves plan mode for the mode you pick in that prompt; `Shift+Tab` or `/plan` plans again.

`downshift setup --no-session-policy` wires the hooks and leaves the three keys alone. To undo it later, delete the keys or run `downshift setup --undo` to restore the last backup. To keep one without the other, set the one you want yourself first: setup keeps it.

The two layers fit together. Claude Code resolves an inheriting subagent's model when it starts it, so a subagent spawned during plan mode would run on Opus. The adapter reads the parent model from the last assistant message in the transcript, which is the resolved model (Opus or Sonnet), not the alias, so the baseline stays known and the subagent is right-sized as usual. This follows from the code path; it has not yet been observed in a live plan-mode spawn.

Pinning a model (`"model": "claude-sonnet-5-5"`, `/model`, `--model` or `ANTHROPIC_MODEL`) opts out of opusplan. `ANTHROPIC_DEFAULT_MODEL=opusplan` is ignored by Claude Code; use the `model` setting. The VS Code extension does not read `permissions.defaultMode`; it uses `claudeCode.initialPermissionMode`.

## Where the list comes from

Same order for every harness:

1. Hook payload, if it includes `session_models` or `available_models` (a JSON array of strings). A present field wins, including an empty array.
2. For Cursor, a configured native export with complete fresh availability. A stale or invalid configured export holds routing. Its provider order is not a capability ranking: resolver tier metadata is required and unknown capability stays ineligible.
3. Otherwise the models recovered for that harness by `downshift models discover` (`discovered.json`). See [session-discovery.md](session-discovery.md). A recovered list beats a handwritten allowlist.
4. Otherwise `~/.downshift/session-models.json` (legacy `~/.harness-downshift/`) (override the path with `DOWNSHIFT_SESSION_MODELS`). An exact `sessions.<harness>.<session_id>` list wins when present; otherwise the top-level harness key is the fallback. This file is not a credit report. `quota` in it is ignored.

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

5. Otherwise, for a harness with `InferSession` in `HarnessCapabilities` (Claude Code), the list is inferred from the current model: the catalog tier ladder below it, ending at the model itself. The current model proves access, so downshift can descend and upshift never passes it. Without a current model the session stays unknown and nothing is rewritten. `DOWNSHIFT_INFER_SESSION=off` disables it. Quota gates still apply.

There is no built-in default list. `docs/examples/session-models.example.json` records one Cursor session from 2026-10-04. Copy it and edit it. Do not expect the binary to load that example on its own.

## How a target is chosen

The session list determines which ids are candidates and their order (least to most capable). Catalog lookup enriches a known ID with metadata, supplies its tier, and preserves `routing: explicit_only`; unknown IDs are not filtered, ranked by guessed names, or replaced.

- Downshift or unknown verdict, catalog path: when every id left after filtering resolves in the catalog (`LookupByID`, which includes family-prefix matches), choose the first id in list order whose catalog tier is at or above the tier the task needs. The position in the list does not matter, so adding a frontier model at the end does not stop a mid task from landing on the mid model. If no id reaches the needed tier, nothing is rewritten; the selector never goes below the floor.
- Downshift or unknown verdict, fallback path: if any id has no catalog tier (an operator id the catalog does not know), the tier cannot rank the list and the position mapping applies on the filtered order: first for small, midpoint for mid, last for frontier.
- The quality floor is set before the quota filter. An exhausted id is skipped, but it never lowers the floor: the next id at or above the needed tier wins, and if every such id is exhausted nothing is rewritten.
- Upshift, catalog path: when every remaining id has a catalog tier, choose the highest tier at or above the current model's tier (later list position first within a tier), skipping ids the quota refuses. If none qualifies, nothing is rewritten. When the catalog cannot rank the list, the position mapping applies: the last quota-allowed id, not earlier than the list midpoint for a mid target. Choosing the cheapest model at the needed tier is an open question (see the [hardening audit](../evidence/hardening-audit-2026-10-15.md)).
- Guessed baselines: when the baseline is inferred rather than requested (Claude Code settings, `BASELINE_FROM_SETTINGS`; Cursor's parent `model_id`), a downshift is written only to the small tier, and a mid target is held. Details in [harness-matrix.md](harness-matrix.md).
- Guardrail hold (`Checked` and `SafeVerdict == OK`): do not rewrite.
- Unknown session (still unknown after step 5, the Claude Code inference from the current model): no rewrite.
- Any exact session ID inside the filtered set can be written, even when the catalog does not know it. `inherit` and exhausted IDs cannot be selected. A catalog `explicit_only` ID can be written only when `explicit_upshift` is on.
- `inherit` may sit in the session list so the payload is recognised. It is never selected as the target.
- Hook credit marks come from this call's `included_models` and `unavailable_models`. Native usage observations are separate, with source, scope and expiry; see [quota.md](../guides/quota.md). There is no compiled-in credit list, and `quota` in `session-models.json` is not read.
- An exhausted id from that hook report is never selected, on any harness.
- When the hook reports `included_models`, downshift and upshift rank only inside that set, in session order, for that harness alone. There is no fallback to a session id outside the set. An empty intersection does not rewrite.
- When the hook did not report credits, ranking stays the session order minus exhausted ids, `inherit`, and `explicit_only` (unless explicit upshift is on). Downshift does not invent a credit denial.

## Observed usage quota

Availability does not establish remaining quota. When a usage snapshot is
supplied by the hook or found in the local quota cache, eligible model IDs must
also have **available** quota evidence for this harness. Unknown, expired,
future-dated or exhausted evidence cannot authorize a rewrite. A shared native
window applies to its harness; a model pool applies only to the exact IDs its
source reports. Percentages never become a guessed list of model names.

`DOWNSHIFT_QUOTA_MODE=required` also holds model rewrites when no quota evidence
exists. The default retains routing compatibility when there is no usage source;
its quota status is **unknown**, so that mode is not a claim of included credit.
A present cache with a missing harness entry remains unknown and holds rewrites.
Hook credit marks and usage quota are intersected, and availability remains a
separate requirement. Neither source may introduce an unavailable model ID.

Codex can collect subscription windows from an explicitly supplied
`transcript_path`, using the event's original timestamp. Collection failures
cannot prove remaining quota. Claude Code supplies its subscription windows via
the optional statusline bridge. Context-window utilization and purchased-credit
balance are distinct from these subscription windows and are not used as quota.

Fresh `used_percent < 100` is an observation of remaining quota; it does not
reserve capacity for the next call or guarantee zero additional charges.

## What Downshift shows while it works

Claude Code shows `systemMessage` text from hooks. Downshift uses it twice:

- when it rewrites a spawn: `downshift: TRIVIAL task → downshift to claude-haiku-4-5 (~75% cheaper than opus)`;
- when a subagent ends and its tokens are known (**SubagentStop** on Claude Code,
  after the child transcript is complete; not PostToolUse on an async launch,
  which has no tokens yet): `↳ downshift: finished on haiku · 14k tokens · $0.01 (opus would be $0.38)`.
  The model is the one the child ran on; the would-be cost is the requested
  model's price for the same tokens, and appears only when that model is known
  and differs. Missing parts are left out, never guessed. PostToolUse at launch
  records `resolvedModel` but prints no finish line until SubagentStop supplies usage.

`DOWNSHIFT_QUIET=1` silences both lines. Routing, telemetry and the stderr notes
are unchanged. The line never blocks, never changes the subagent result, and a
failed hook prints nothing. The harness fact is `CanShowChildLine` in
`HarnessCapabilities` (true for Claude Code only, false elsewhere until a
harness is shown to display it); the line is built by `telemetry.ChildEndLine`
for every harness. Not yet verified: that a given Claude Code release displays
`systemMessage` from PostToolUse and SubagentStop (documented as a common hook
output field, not observed on a live session).

## Capability path (public scores)

An optional second way to pick a model from the same session list. It chooses
the cheapest candidate whose public capability score reaches the level the task
tier needs, measured across the session's own range. It never adds a candidate
id, it sits behind the same guardrails and quota gates, and it abstains on
COMPLEX work, any risk signal, any deterministic lift, and when any candidate
lacks a score. Full rules, sources and privacy: [scores.md](scores.md).

- Default: shadow. The pick is recorded (`score_would`, `score_differs`,
  `score_would_cost_ratio`) and the routed model does not change.
- `DOWNSHIFT_SCORE_ROUTING=1`: the pick is applied when the plan rewrites the
  model, with reason code `SCORE_ROUTED`.
- `DOWNSHIFT_SCORES=off`: no scores are read, so the capability path never picks.
- Which session ids have a score today: [harness-matrix.md](harness-matrix.md#public-score-coverage-capability-path).

## explicit_upshift

Default off. Without it, `explicit_only` models do not participate in upshift. With it, upshift (`VerdictUpshift` only) may select them. The last remaining id in the filtered session order wins. Downshift and ordinary tier mapping still skip `explicit_only`. If the current model is already `explicit_only`, it stays; the flag does not replace it.

An `explicit_only` id is selectable for upshift only when the flag is on and, if a credit set exists, the id is inside that set. `CanWriteSessionID` refuses the write otherwise, so the hook does not emit the upshift.

Turn the flag on with any of:

- `"explicit_upshift": true` in `session-models.json`
- `DOWNSHIFT_EXPLICIT_UPSHIFT=1`
- `install.sh --explicit-upshift` (the version argument still works before or after the flag)

Cursor `claude-fable-5-1-thinking-high` and Codex `gpt-6-astra` are `explicit_only`. Claude Code `claude-fable-5-1` is not: list that id in the session when you want upshift to reach it. Do not alias another harness's canonical id.

For example, a list `["session-small", "session-mid", "session-frontier"]` makes those three positions available to all routing decisions without requiring any of the IDs or their prices in Downshift's catalog. If the session list is missing, the active model stays.
