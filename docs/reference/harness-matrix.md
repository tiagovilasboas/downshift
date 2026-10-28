# Harness x plan x rewrite-honored matrix

Downshift by Tiago Vilas Boas
https://github.com/tiagovilasboas/downshift

## Status per harness

Each harness carries exactly one of four status words (owner decision 2026-10-17; Beta added 2026-10-18); the table below it keeps the per-plan facts.

- **Completed:** rewrite observed honored on a real child, usage and child end recorded, plugin and session policy.
- **Functional:** the hook emits the rewrite on a real spawn and passes the shared conformance suite; observation is partial (honored is inferred or unconfirmed, no child usage).
- **Beta:** first-class integration under active development — adapter, route wire and shared conformance in the repository and green; the harness-side plugin and the first live observation are still ahead, so no live spawn is observed yet and there is no production claim (owner decision 2026-10-18).
- **Experimental (proof of concept):** policy mode or one observed child on one build; shipped, but no production claim.

`cmd/downshift/conformance_test.go` drives the same 8 routing scenarios against every registered hook, so each harness below, whatever its status, computes every feature (layering rule); the status says how much has been verified on the real harness, not how much code it runs.

| harness | status | why | evidence |
|---|---|---|---|
| claude-code | Completed | Rewrite observed honored (2026-10-06, API-reported child model); usage tokens read from the child transcript at `SubagentStop`; child end recorded; plugin and session policy ship. Failure payload is still GAP-CHILD-FAILURE (see "Child end" below). | [rewrite honored](../evidence/claude-code-rewrite-honored-2026-10-06.md), [billing audit](../evidence/billing-gap-audit.md), [routing A/B](../evidence/subagent-routing-ab-2026-10-15.md) |
| codex | Functional | Rewrite emitted on a real spawn; honored is inferred from the next spawn, not observed; child end parser unverified, no usage source. | [codex rewrite](../evidence/codex-rewrite-honored-2026-10-02.md) |
| cursor | Functional | Rewrite emitted on a real spawn; the child executor model is absent from the transcript, so honored is unconfirmed (discarded on Free and legacy Pro); end parser unverified, no usage; native quota collector blocked on Cursor 3.24.9. | [cursor rewrite](../evidence/cursor-rewrite-honored-2026-10-10.md), [gap diagnostic](https://github.com/tiagovilasboas/downshift-labs/blob/main/reverse/cursor/findings/F1-cursor-gap-diagnostic.md) |
| kirocrew | Experimental | Policy mode: block and retry observed once on Kiro CLI 2.29.0, no in-place rewrite; child end via postToolUse (unverified: needs real payload). | [native hook](https://github.com/tiagovilasboas/downshift-labs/blob/main/reverse/kirocrew/findings/F5-kirocrew-native-hook.md) |
| antigravity | Experimental | One observed child on desktop 2.21.1 (requested `pro`, executed `flash_lite`); the `overwrite` envelope is not a documented vendor field; no child end recorded; quota mapping unresolved. | [executor ack](../evidence/antigravity-executor-ack.md), [runtime source](https://github.com/tiagovilasboas/downshift-labs/blob/main/reverse/antigravity/findings/F1-antigravity-runtime-source.md) |
| opencode | Beta | First-class integration under active development (owner decision 2026-10-18): adapter, route wire v1 and shared conformance in the repository; the forwarding plugin is in development and no live spawn is observed yet. Route answers, fail-open paths and observation parsing are covered by adapter tests and the shared conformance suite. No child end recorded yet; usage seam not proven. | [adapter tests](../../internal/adapters/opencode/), [conformance](../../cmd/downshift/conformance_test.go) |
| grok | Config only | No hook rewrite; reasoning is pinned in `config.toml`. | [install.md](../guides/install.md#grok-cli-config-not-hook) |

"Rewrite honored" means the harness executor applied the hook's model
rewrite to the child subagent, not just that downshift emitted one.
`outcome: "rewrite_emitted"` proves emission only (docs/reference/session-models.md:17).

| harness | plan / build | rewrite honored? | evidence / limit |
|---|---|---|---|
| claude-code | paid: Pro / Max / Teams / API | Yes, 2026-10-06 (two spawns; API-reported model in the child transcript) | `updatedInput.model` must be a family name (`haiku`/`sonnet`/`opus`/`fable`); a full id is rejected and blocks the spawn. The catalog `native_name` supplies it. Write-up: `docs/evidence/claude-code-rewrite-honored-2026-10-06.md`. Built-in agent types (`Explore`, `claude-code-guide`, `statusline-setup`) route to the `small` tier (Haiku) by default profile (W7.1), with risk guard elevation to `mid`. Setup configures `CLAUDE_CODE_SUBAGENT_MODEL_FORCE=1` in `~/.claude/settings.json` so Claude Code honors subagent model rewrites. Economy mode (`downshift policy economy on`) downshifts all profiles one tier within safety boundaries. The payload carries no session list; without one (payload, discovery or `session-models.json`) the catalog `session_candidates` below the current model stand in for it (`SessionFromCatalog`, downshift only, never above the current model); a list is needed only to allow upshifts. |
| claude-code | Free | No: no real subagents | Sandboxed, no egress; subagents need paid (README.md:728) |
| cursor | Free | No: silently discarded | `updated_input.model` ignored, known issue (README.md:734) |
| cursor | Pro legacy (request-based) | No: silently dropped | Task model takes only `"fast"` (README.md:735) |
| cursor | Pro / Ultra usage-based | Unconfirmed | Actual child-model proof remains P1.6. Separately, Cursor 3.24.9 blocks the external native quota collector before transport; see [diagnostic](https://github.com/tiagovilasboas/downshift-labs/blob/main/reverse/cursor/findings/F1-cursor-gap-diagnostic.md). |
| codex | multi_agent_v2 + hooks on | Yes, inferred (2026-10-02) | Inferred, not observed: same-session follow-up: spawn requested `gpt-5.6-terra` after hook emitted Terra (`docs/evidence/codex-rewrite-honored-2026-10-02.md`). Payload >1MB still fail-open. Schema still evolving upstream. |
| kirocrew | Kiro CLI 2.29 native spec hook; MCP `spawn_run` with explicit `task` + `model`, or `spawn_run` with `tasks[]` (each entry `{task, model, reasoning_effort}`) | Policy/block + retry observed 2026-10-10; no in-place rewrite | Persistent `agent.kiro_hooks.preToolUse` + executable wrapper; no `updated_input` channel. Native `use_subagent` without child model is outside coverage. `hooks.json` tool-call hooks are informational and do not enforce blocks. [Native evidence](https://github.com/tiagovilasboas/downshift-labs/blob/main/reverse/kirocrew/findings/F5-kirocrew-native-hook.md); [setup and limits](../guides/install.md#kirocrew-policy-mode-through-the-native-kiro-cli-hook). `spawn_run` `tasks[]`: each entry is routed on its own, with one decision and one telemetry row per entry; a block names every entry to respawn and the whole call is blocked (KiroCrew cannot rewrite one entry). Batch rows carry no top-level model, so their telemetry `FromModel` is `unknown`. A top-level `spawn_run` without `tasks` routes as before. `spawn_sub_agents` is **not routed** (passes through unchanged). Shape verified in the KiroCrew 2.29 backend source (`validation.py` `SPAWN_SUB_AGENTS_SCHEMA`, `mcp_tools/spawn.py`), not a captured payload: `agents: [{agent_or_mode, prompt}]` with no per-entry model; the handler forwards only task, agent and context flags, so the child model is set by the named agent. Downshift cannot route it. The postToolUse matcher stays `spawn_run`. Table tests: `internal/adapters/kirocrew/batch_test.go`. |
| antigravity | desktop 2.21.1, `invoke_subagent` | Yes, observed 2026-10-09 | One exact child: original `pro`, executed `flash_lite`, native executor/generation model `gemini-3.5-flash-lite`. [Evidence and QA](../evidence/antigravity-executor-ack.md). Manual read-only, build-specific sensor; quota/picker/alias mapping remains unresolved. |
| opencode | zen models (no published list price) | Not observed yet | Adapter and conformance only (the forwarding plugin is phase 2). The route answer carries the target model; the observation side reports a resolved model only from an arrival-ordered switch chain (`switch_attempt` ok, then `model_selected`, then the first `child_context`) compared against the decision target. Usage seam not proven: no cost recorded. |
| grok | any (single model grok-4.6) | No hook rewrite: config only | PreToolUse is allow/deny; pin `reasoning_effort` in config.toml (README.md:520-531) |

Packaging note: the plugin (`.claude-plugin/`, `hooks/hooks.json`, `agents/`)
loads only on Claude Code. Every other harness gets the same hooks through
`downshift setup` or the manual sections of
[install.md](../guides/install.md); the skills under `skills/` are plain
SKILL.md files any harness can read.

"Inferred" means a later spawn in the same session asked for the model the
hook had written (`rewrite_honored_inferred` in `stats --export`). It is
correlation, not a read of the child's actual model. Only applied rewrites
(outcome `rewrite_emitted`, no guardrail correction) count; allow, blocked
and held events never do.

Per-harness measurement (what is observed, what blocks per-token figures, which catalog prices apply, and catalog anomalies):
[value-per-harness.md](value-per-harness.md).

## Decision linkage (current model and post-spawn cost)

Observed on claude-code 2026-10-12: the PreToolUse payload carries
`tool_use_id` and `transcript_path` but no session model.

- **Current model.** A Task call without `model` (agent type empty,
  `general-purpose`, `claude`, and the built-in `Explore` and `Plan`) runs on
  the session model. The adapter reads it from the last assistant message in
  the tail of `transcript_path` (1 MiB, model id only). Any other type is decided
  by its definition (2026-10-19): a built-in that runs on its own model
  (`statusline-setup` on Sonnet, `claude-code-guide` on Haiku) and an agent
  file whose `model:` names one are a known baseline that routing leaves
  alone (`AGENT_PINS_MODEL`); an agent file with no `model:` or `inherit` (project
  `.claude/agents`, user `agents`, or an installed plugin's `agents`, found through
  `installed_plugins.json` for a `plugin:agent` type) inherits like
  `general-purpose`. A type with no definition found stays `unknown`. `Explore`
  also stays `unknown` when the session model is Fable: Claude Code may run it
  on Opus (subscription, Console, gateway) or keep it on the main model (cloud
  providers), and a hook cannot tell which. Until 2026-10-15 `Explore` and
  `Plan` were left out, so every spawn of those types was held as
  `HELD_UNCONFIDENT_UNKNOWN_MID` and never routed.
- **Claude Code: parallel Agent calls in the first assistant message.** The hook gets the session model from the
  last assistant message in the transcript, and Claude Code only writes a message to the transcript when it has finished streaming.
  When one message carries several Agent calls and no earlier assistant message exists (a headless run whose first action is to
  delegate), the hooks of all but the last call see an empty transcript. Measured in 3 runs of 5 parallel Agent calls: 4 of 5 spawns
  per run had no baseline and were held (`HELD_UNCONFIDENT_UNKNOWN_MID`). The 400 ms wait added for the write race does not help,
  because the message closes only after all its tool-use blocks have streamed, and `SessionStart` does not carry the model in
  `claude -p` either (checked 2026-10-15). Now, for an agent type that inherits the session model, when the transcript has no model
  the adapter falls back to the model the session is configured to use: `ANTHROPIC_MODEL`, then `model` in
  `<cwd>/.claude/settings.local.json`, `<cwd>/.claude/settings.json` and the user `settings.json` (`CLAUDE_CONFIG_DIR` honoured), first
  one that sets it. A full id passes through, `opusplan` is Opus under `permission_mode` `plan` and Sonnet otherwise, `sonnet`, `opus`,
  `haiku` and `fable` map to the catalog's model of that family, and `default` or anything unrecognised stays unknown. The decision
  carries `BASELINE_FROM_SETTINGS`. The result is an inference: the `--model` flag and a mid-session `/model` are not visible to a
  hook, so a session started with `--model` or switched with `/model` can be routed against the wrong baseline. To keep that guess
  from turning into a hidden upshift, a downshift on this baseline is written only to the small tier; a downshift to the mid tier is
  held. A model found in the
  transcript always wins, `Explore` under a Fable setting stays unknown, and agent types that do not inherit get no fallback. The
  native route remains the `agent.spawn` mod event, which hands over `parentModel`.
- **Baseline still unknown on Antigravity; Cursor `inherit` uses the parent when
  present.** A spawn with an explicit model is known. Antigravity's `inherit` has
  no parent model in the payload. Cursor sends no transcript path (58 of 144
  captured Task payloads, 2026-10-09/10). When a Cursor call has no
  `tool_input.model` or it is `inherit`, the parent's `model_id` (then payload
  `model`) is the baseline, and a downshift on it is written only to the small
  tier: a child can run on Composer or a fast variant, so a mid target could be
  an upshift. Without `model_id`, `inherit` logs as `unknown`.
- **Upshift ranking, all harnesses.** An upshift ranks the session ids by catalog tier, not list position, and never goes below the
  current model's tier. A confident upshift lands on the least model at the needed tier; a guessed baseline (Claude Code settings, Cursor
  parent `model_id`) or `explicit_upshift` keeps the strongest tier. Codex, Antigravity and KiroCrew never guess a baseline, so they always
  take the needed-tier pick. The rule and its fallback are in [session-models.md](session-models.md).
- **Cost linkage.** Every decision records `tool_use_hash`. PostToolUse
  repeats the id, so `resolved`, `child_ended` and, through the agent id,
  `usage` attach to the exact decision even with parallel spawns, including
  held and allow decisions (no `rewrite_honored` claim for those), and
  control-group and no-route decisions (`outcome: baseline`): before
  2026-10-19 the linker treated `baseline` as a cost record and skipped it, so
  no control spawn ever had an end record. Lines written before this field
  fall back to the session heuristic.
- **One spawn, two harnesses (all harnesses).** Cursor runs the Claude Code
  hooks in `~/.claude/settings.json` when its third-party configs are on
  ([Cursor docs](https://cursor.com/docs/reference/third-party-hooks)), so one
  Cursor spawn reaches both the cursor hook and the claude-code hook. Each
  harness declares `PayloadMarkers` (top-level keys only it sends; Cursor:
  `cursor_version`, `conversation_id`, `generation_id`, `workspace_roots`), and
  the shared hook, honor, usage and end runners leave a payload carrying
  another harness's marker to that harness's own hook: allowed unchanged,
  nothing recorded, one stderr line. Before 2026-10-19 the copy was routed and
  logged as a claude-code decision with Cursor's `inherit` and lowercase agent
  types: all 79 claude-code decisions labelled `agent_type: custom` in the
  operator's 30-day log were such copies (each within 3 s of a cursor decision;
  all 33 with a tool-call id shared it). Not verified on a live Cursor build:
  that the copy carries at least one marker. It is the native Cursor input, and
  the logged copies match it.
- **Custom agent types on Claude Code.** A real custom type (project or user
  `.claude/agents/<name>.md`, or a plugin agent `plugin:name`) still logs an
  unknown baseline unless the call names a model: the payload carries only the
  type name, and the model its definition pins (or `inherit`) is in a file the
  hook does not read. Reading it is option 2 of
  [subagent-type-signal.md](../design/subagent-type-signal.md), held back
  until it is measured. Built-in pinned types (`claude-code-guide`,
  `statusline-setup`) ship no definition on disk, so they stay `unknown` too.
- **Other harnesses.** cursor and codex expose `tool_use_id` on decisions
  (documented in their hook payloads). Their end hooks (see "Child end" below)
  join on it when the end payload repeats the id.
- **codex current model.** A `spawn_agent` call without `model` runs on the
  parent's current model: 22 of 24 such spawns in the operator's rollouts
  (2026-10-12; the other mismatches were Downshift's own rewrites). The
  adapter reads the last `turn_context.payload.model` from the tail of
  `transcript_path`. `gpt-6.1-sol` is registered for identity and tier and has
  list prices since #190 (2026-10-19), so cost telemetry can price it once a
  token source exists (Codex has none today). `gpt-reserve` is not in the catalog yet, so they
  still log as `unknown` with `requested_model_raw` set.
- **antigravity current model.** `Subagents[].Model` arrives in the payload
  (2026-10-09 run asked for `pro`) but was never exposed to telemetry; it is
  now, for the subagent whose decision is recorded (the last with a prompt).
  The documented top-level `modelName` is not read until a real payload
  confirms it.
- **cursor current model.** In 144 captured Task payloads (2026-10-09/10) the
  top-level `model` was always empty; the subagent `model` was a real id,
  `inherit` (28) or absent (30). With `model_id` on the payload, `inherit` is
  resolved like a missing `tool_input.model` and logged as that parent id. On
  Cursor 3.27.12 (2026-10-24) Task `preToolUse` often omits `model_id`; when
  `transcript_path` is present the adapter reads the Claude-shaped parent JSONL
  (last assistant `message.model`, or the model on the line that carries this
  `tool_use_id`), bounded wait as Claude Code. Without payload model fields or
  a readable transcript, baseline stays `unknown`.
- **Unrecognised models (all harnesses).** When the requested model is
  non-empty but not in the catalog, the shared runner records it in
  `requested_model_raw` if it looks like a model id (64 chars,
  `[A-Za-z0-9._:/-]`), else `invalid`.

## What each harness exposes for session discovery (2026-10-11/12)

Findings from the official docs plus local read-only inspection. Rows say what a
hook or local source can tell Downshift; anything not listed was not found.
`SessionFromCatalog` (HarnessCapabilities) is on only where the payload sends no
session list and every session accepts the catalog `session_candidates` names
(Claude Code family aliases). A list from the payload or discovery wins.

| harness | current model in hook | available models | quota | session-start hook | SessionFromCatalog |
|---|---|---|---|---|---|
| claude-code | `SessionStart.model` (optional); the Agent `tool_input.model` when the parent set it; `PostToolUse.tool_response.resolvedModel` | No listing for subscriptions; `availableModels`/`deniedModels` exist only in managed settings; the Models API needs `ANTHROPIC_API_KEY` | statusline `rate_limits.five_hour` / `seven_day` (global, Pro/Max only, after the first response); not per model | `SessionStart` (add context, `CLAUDE_ENV_FILE`) | yes |
| codex | `model` on every payload; effort only if the parent put `reasoning_effort` in `spawn_agent` | `models_cache.json` (offline; `hide` means not shown in the picker, not unusable); app-server `model/list` is experimental | per meter (`codex`, `premium`), never per model | `SessionStart` can only add context | no |
| cursor | `model`, `model_id`, `model_params`; `transcript_path` on `preToolUse` (3.27+); `subagent_model` on `subagentStart` (observe only, no rewrite) | `cursor-agent models` (text, needs login; failed here with "Authentication required"); Cloud `/v1/models` needs an API key | Cursor Models vs Other Models pools, UI/dashboard only; CLI `/usage` is interactive | `sessionStart` (env reaches later hooks; not for cloud agents) | no |
| antigravity | `modelName` (parent) and `conversationId`; the adapter does not read them yet | `agy --output-format json models` (parent picker); no alias-to-model or pool mapping | `agy -p /usage --output-format json` per account, group and window; not tied to an alias | none (Pre/PostToolUse, Pre/PostInvocation, Stop) | no |
| kirocrew | none in the payload; `session_id` maps to `~/.kiro/sessions/cli/<id>.json` (`agent_name`, `model_id`, undocumented) | `kiro-cli chat --list-models --format json` (10 models plus `auto`, with `rate_multiplier`) | no supported non-interactive credit source | `agentSpawn` (renamed `SessionStart` in V3); stdout goes to the agent context; any non-zero exit blocks | no |

Consequences: Claude Code is the only harness where the catalog candidates are
safe (restored 2026-10-27 after PR #570 removed the inferred ladder and left
every Claude Code spawn unknown); every other harness needs a real list (hook payload, discovery cache or
the operator file). Quota stays `unknown` everywhere except through the Claude
Code statusline bridge and operator imports. Re-verify per release.

## Child end (N5, 2026-10-13)

One shared recorder (`internal/childend`) writes `resolved`, `child_ended` and
`usage` for any harness from a `hookport.ChildEnd`. An adapter only parses its
payload (`ParseChildEnd`); the CLI entry is `downshift end <harness>`, and
`setup` wires it from `HarnessCapabilities.ChildEndEvent`. `downshift report`
and `downshift doctor` print, per harness, whether completion is measurable
(an end event is declared and at least one end record has been seen; a launch is
not an end). Claude Code keeps `claude-code-post-tool-use` and
`claude-code-subagent-stop` as aliases of the same runner.

| harness | `ChildEndEvent` | `UsageSource` | status |
|---|---|---|---|
| claude-code | `SubagentStop` (launch and failure also on `PostToolUse` / `PostToolUseFailure`) | transcript (`agent_transcript_path`) | verified 2026-10-06 (launch payload, SubagentStop transcript); failure payload still GAP-CHILD-FAILURE |
| codex | `PostToolUse` for `spawn_agent` | **none** (`UsageSource: none`): end and status are recorded, cost is not. A parser for the rollout `token_count` exists but stays off until a real payload proves `transcript_path` is the child's rollout | **unverified: needs a real payload.** `tool_response.status` / `model` are assumed names; the `token_count` usage shape is from the Codex rollout format, not from a captured hook. A spawn that reports no status is a launch: it records `launched`, reads no usage and does not make completion measurable. `transcript_path` is the parent rollout, so until a real payload shows a child-scoped rollout the tokens may be the parent thread's. |
| cursor | `subagentStop` | none (no usage field is read) | **verified on 3.27.12 (2026-10-24):** `subagentStop` carries `status`, `session_id`, and `subagent_id` (matches Task `tool_use_id`; no `tool_call_id`). PreToolUse on the same build sends `session_id` and `tool_use_id` but leaves `model`, `model_id`, and `tool_input.model` empty, so routing baseline stays unknown and R9 may hold. No executed child model field is read. |
| antigravity | none | none | no documented subagent-end payload (PostToolUse exists; its result fields are not documented). Nothing recorded. |
| kirocrew | none | none | PostToolUse `tool_response` not confirmed (see `evidence/vendor-hook-docs-2026-10-11.md`). Nothing recorded. |
| opencode | `child_done` (observation envelope the plugin forwards) | none (`UsageSource: none`): the `child_done` parser reads status and inline usage counts, but nothing is priced. | **unverified: needs a live run.** The envelope shape comes from the phase-0 probes, not from a captured plugin forward. The honor reader reports a resolved model only from the arrival-ordered switch chain; a batch without it records nothing. |

Where `ChildEndEvent` is empty, `downshift end <harness>` exits 0, records nothing
and says so on stderr.

## Child-end line (U2, 2026-10-13)

`HarnessCapabilities.CanShowChildLine` says whether the hook protocol can show
one line when a subagent ends. Claude Code: true (`systemMessage` on
PostToolUse and SubagentStop), pending a live check. cursor, codex, antigravity,
kirocrew, opencode: false; the line is still computed by `telemetry.ChildEndLine` (by the
shared recorder, for every harness) and no adapter prints it. `DOWNSHIFT_QUIET=1` turns it off. See
[session-models.md](session-models.md#what-downshift-shows-while-it-works).

## Subagent type hint (`AgentTypeHints`)

`HarnessCapabilities.AgentTypeHints` maps a subagent type, exactly as the harness
sends it, to a tier. `core.RouteAgent` uses it only to fill doubt: the text read
must be unconfident, with no risk signal, no NB or semantic lift and no COMPLEX
read, the current model must be known, and the hint is capped at that model's
tier (no upshift). Reason code `AGENT_TYPE_HINT`. Every harness computes it the
same way; the table is data.

| harness | type field read | hints | status |
|---|---|---|---|
| claude-code | `tool_input.subagent_type` | `Explore` small, `Plan` mid | from the built-in types' documented jobs; pinned built-ins carry none |
| cursor | `tool_input.subagent_type` | none | field read, no stable names observed yet (lowercase `explore` seen); add only from evidence |
| codex | `tool_input.agent_type` | none | field read (seen in spawn inputs); no hint names until evidence |
| kirocrew, antigravity, opencode | not read | none | no type field in the adapter payload |

## Spawn state (2026-10-19)

`switched` / `held` / `observed` is computed in `telemetry.SpawnStateOf` and
shown by the shared runner for every harness: one stderr line per spawn, plus
`live.json` for panels. No harness-specific code. Limits: the stderr line only
helps where the harness surfaces hook stderr (unverified per harness), and the
statusline segment needs a statusline bridge (Claude Code today). Details:
[spawn-state.md](spawn-state.md).

## Session view (U3, 2026-10-13)

`downshift session` (tree, `--why`, `--pause`) is shared code; what differs is
whether a harness gives it a session id. `HarnessCapabilities.SessionEnv` names
the variable a harness exports to commands run from it: Claude Code
`CLAUDE_CODE_SESSION_ID` (seen set in a Bash tool call on Claude Code 2.1.289)
and KiroCrew `KIRO_SESSION_ID` (set by the KiroCrew gateway for every agent
process, PR #559, 2026-10-25); Cursor, Codex and Antigravity declare none and
the command uses the latest session in the log. The hook payload carries a
session id for Claude Code and Codex only, so only they group in the tree and
can be paused per session. Antigravity, Cursor and KiroCrew carry none in the
hook payload: computed and logged, not groupable, not pausable (the global
`DOWNSHIFT_NO_ROUTE=1` still applies). KiroCrew's `SessionEnv` does let a
command run from the harness (the `downshift compactor`/`exec` shell wrap)
attribute its exec event to `kirocrew` instead of `unknown`, through
`envHarness()` reading `KIRO_SESSION_ID`; the tree grouping still needs a
hook-payload id it does not have. The `/downshift` slash command is Claude
Code only. See [session-view.md](../guides/session-view.md).

## Bundled and sibling mods (2026-10-17)

The Claude Code plugin's `hooks/hooks.json` lists one mod module,
[`hooks/cache-tax/`](../../hooks/cache-tax/README.md) (third-party, MIT,
vendored and pinned; owner decision 2026-10-17). One more vendored mod ships in
the same marketplace as a sibling plugin, one install, because a plugin
loads a single hooks module and both hook `session.start`:
[`mods/savvy-progress/`](../../mods/savvy-progress/README.md)
(`savvy-progress@downshift`); see [`mods/README.md`](../../mods/README.md). None is a
routing feature and nothing in the shared layers computes it: a mod is code
Claude Code runs in its own process, so it exists only where mods load.

| harness | mods | why |
|---|---|---|
| claude-code | cache-tax loads with the plugin on 2.1.287+; savvy-progress is a separate plugin install (CLI, Desktop Code tab, VS Code extension; hooks also run in `claude -p`, nothing is drawn there) | mods are a Claude Code plugin feature; `setup` cannot write one |
| codex, cursor, kirocrew, antigravity, grok | not available | no mod system; the cold-cache question is the harness's own |

What each does is in its README; the one fact that matters here is that the
cache-tax warming ping spends usage and is armed by the user (or for three hours
after a detected cold write), while the guard itself sends nothing.

## Answer when nothing is rewritten (2026-10-19)

An allow on an unchanged spawn can override the user's own ask rules, so the shared
runner answers neutrally where the host permits it (PR #192) and with the host's
required decision where it does not.

| Harness | Unchanged spawn | Why |
|---|---|---|
| claude-code, codex | neutral `{}` | `permissionDecision` is optional; `updatedInput` is only accepted together with `allow`, so `allow` is emitted only with a rewrite. Pinned by `TestHarnessConformance` ("unchanged spawn must emit no permissionDecision"). |
| antigravity | `{"decision":"allow"}` | The vendor summary lists `decision` as **required** output (allow, deny, ask, force_ask, deny_unless_prior_grant) ([vendor-hook-docs](../evidence/vendor-hook-docs-2026-10-11.md)). A neutral object would be a missing required field. Not changed. |
| cursor | `{"permission":"allow"}` | The vendor summary documents `permission` allow/deny and says an invalid response blocks the action; it does not say the field is optional, and no capture of a neutral `{}` exists. Not changed. |
| opencode | `{"v":1,"action":"allow",...}` | The envelope's `action` is required by the plugin wire contract. |
| kirocrew | empty stdout, exit 0 | exit-code contract. |

Whether cursor or antigravity treat an explicit allow as overriding a user prompt
rule has **not** been verified on a live build (no Cursor or Antigravity session was
driven). To change either, first capture how the host treats `{}` for a spawn it has
a rule for, then add the harness to the neutral set and extend the conformance
decoders. Until then the safe answer is the documented one.

## Shell command rewrite: `downshift compact` (on by default, 2026-10-19)

On by default on Claude Code: the plugin registers the `Bash` PreToolUse hook. Set
`"exec": false` in the Downshift config or `DOWNSHIFT_EXEC=0` to turn it off. The decision is
shared (`internal/execwrap.Decide` plus `shellWrapDecision` in the hook
runner): the command must start with one of the families below, contain no
shell metacharacter (or match one of the compound shapes below), not already start with `downshift`, and be rewritten to the
running binary's absolute path (single-quoted in the command, no PATH lookup).
No Bash allow rule is needed. The rewrite is skipped when a deny or ask rule
matches the command, or when the settings cannot be read, and never when
`"exec": false` or `DOWNSHIFT_EXEC=0` is set. The harness fact is
`HarnessCapabilities.CanRewriteShellCommand`. An adapter takes part by
exposing `ShellCommand()` and `ShellRewrite()`.

**Accepted families.** `FamilyOf` in `internal/execwrap/policy*.go` decides. The
first words must match one of these forms (the inventory below is checked against the code by `internal/execwrap/families_inventory_test.go`; the policy files decide the exact flags and subcommands); words after them are passed on as
written. The family name is the only command fact telemetry keeps.

| family | command forms | behaviour |
|---|---|---|
| `go-test` | `go test` | compresses |
| `pytest` | `pytest` | compresses |
| `cargo-test` | `cargo test` | compresses |
| `flutter-test` | `flutter test` | compresses |
| `dart-test` | `dart test` | compresses |
| `flutter-build` | `flutter build` | passes through |
| `flutter-run` | `flutter run` | passes through |
| `flutter-analyze` | `flutter analyze` | passes through |
| `flutter-pub` | `flutter pub` | passes through |
| `flutter-doctor` | `flutter doctor` | passes through |
| `flutter-clean` | `flutter clean` | passes through |
| `flutter-devices` | `flutter devices` | passes through |
| `dart-run` | `dart run` | passes through |
| `dart-analyze` | `dart analyze` | passes through |
| `dart-pub` | `dart pub` | passes through |
| `dart-format` | `dart format` | passes through |
| `dart-compile` | `dart compile` | passes through |
| `dart-fix` | `dart fix` | passes through |
| `gradle` | `gradle`, `gradlew`, `./gradlew` with the allowlisted tasks (build, check, test, assemble, lint, ...) | compresses |
| `maven` | `mvn`, `mvnw`, `./mvnw` with test, verify, package, install or clean | compresses |
| `phpunit` | `phpunit`, `vendor/bin/phpunit`, `./vendor/bin/phpunit`, `php vendor/bin/phpunit`, `php ./vendor/bin/phpunit` | compresses |
| `composer` | `composer` | passes through |
| `artisan` | `php artisan <command>` | passes through |
| `phpstan` | `phpstan` | passes through |
| `psalm` | `psalm` | passes through |
| `php-cs-fixer` | `php-cs-fixer` | passes through |
| `pint` | `pint` | passes through |
| `pest` | `pest` | passes through |
| `rector` | `rector` | passes through |
| `sail-test` | `sail test` | passes through |
| `sail-artisan` | `sail artisan <command>` | passes through |
| `vitest` | `vitest`, `npx vitest`, `pnpm exec vitest`, `yarn vitest` | compresses |
| `jest` | `jest`, `npx jest`, `pnpm exec jest`, `yarn jest` | compresses |
| `playwright` | `playwright test`, `npx playwright test`, `pnpm exec playwright test`, `yarn playwright test` | compresses |
| `cypress` | `cypress run`, `npx cypress run`, `pnpm exec cypress run`, `yarn cypress run` | compresses |
| `tsc` | `tsc`, `npx tsc`, `pnpm exec tsc`, `yarn tsc` | compresses |
| `next-build` | `next build`, `npx next build`, `pnpm exec next build` | compresses |
| `vite-build` | `vite build` | passes through |
| `vite-preview` | `vite preview` | passes through |
| `turbo-run` | `turbo run` | passes through |
| `npm-install` | `npm install` | passes through |
| `npm-test` | `npm test` | passes through |
| `npm-build` | `npm build` | passes through |
| `pnpm-test` | `pnpm test` | passes through |
| `pnpm-build` | `pnpm build` | passes through |
| `yarn-test` | `yarn test` | passes through |
| `yarn-build` | `yarn build` | passes through |
| `bun-test` | `bun test` | passes through |
| `bun-build` | `bun build` | passes through |
| `eslint` | `eslint`, `pnpm exec eslint`, `yarn eslint` | compresses |
| `prettier` | `prettier` | passes through |
| `biome` | `biome` | passes through |
| `mysql-query` | `mysql -e <read-only statement>` | passes through |
| `redis-read` | `redis-cli <read-only command>` (and `valkey-cli`) | passes through |
| `kubectl-get` | `kubectl get` | passes through |
| `kubectl-describe` | `kubectl describe` | passes through |
| `kubectl-logs` | `kubectl logs` | passes through |
| `helm-list` | `helm list` | passes through |
| `helm-status` | `helm status` | passes through |
| `terraform-plan` | `terraform plan` | passes through |
| `terraform-fmt` | `terraform fmt` | passes through |
| `az-list` | `az list` | passes through |
| `az-show` | `az show` | passes through |
| `aws-describe` | `aws describe` | passes through |
| `aws-list` | `aws list` | passes through |
| `aws-get` | `aws get` | passes through |
| `aws-s3-ls` | `aws s3 ls` | passes through |
| `aws-s3-cp` | `aws s3 cp` (read-only forms only) | passes through |
| `aws-logs` | `aws logs` | passes through |
| `docker-ps` | `docker ps` | passes through |
| `docker-logs` | `docker logs` | passes through |
| `docker-images` | `docker images` | passes through |
| `docker-build` | `docker build` | passes through |
| `argocd-list` | `argocd list` | passes through |
| `argocd-get` | `argocd get` | passes through |
| `argocd-diff` | `argocd diff` | passes through |
| `kustomize-build` | `kustomize build` | passes through |
| `az-pipelines` | `az pipelines <read form>` | passes through |
| `logcli` | `logcli` | passes through |
| `sentry-cli` | `sentry-cli` | passes through |
| `grafana-cli` | `grafana-cli` | passes through |
| `datadog-ci` | `datadog-ci` | passes through |
| `git-status` | `git status` | compresses |
| `git-log` | `git log` | passes through |
| `git-diff` | `git diff` | compresses |
| `git-show` | `git show` | compresses |
| `git-branch` | `git branch` | passes through |
| `git-fetch` | `git fetch` | passes through |
| `git-pull` | `git pull` | passes through |
| `git-push` | `git push` | passes through |
| `git-commit` | `git commit` | passes through |
| `git-add` | `git add` | passes through |
| `git-stash` | `git stash` | passes through |
| `git-remote` | `git remote` | passes through |
| `git-tag` | `git tag` | passes through |
| `git-config-read` | `git config --get`, `--get-all`, `--get-regexp`, `--list` (read flags only) | passes through |
| `git-other` | any other `git <subcommand>` not listed above | passes through |
| `gh-pr` | `gh pr` | passes through |
| `gh-run` | `gh run` | passes through |
| `gh-api` | `gh api` | passes through |
| `gh-issue` | `gh issue` | passes through |
| `gh-repo` | `gh repo` | passes through |
| `gh-auth` | `gh auth` | passes through |
| `gh-search` | `gh search` | passes through |
| `go-build` | `go build` | compresses |
| `go-run` | `go run` | passes through |
| `go-install` | `go install` | passes through |
| `go-mod` | `go mod` | passes through |
| `go-generate` | `go generate` | passes through |
| `go-fmt` | `go fmt` | passes through |
| `go-vet` | `go vet` | passes through |
| `go-doc` | `go doc` | passes through |
| `go-version` | `go version` | passes through |
| `go-env` | `go env` | passes through |
| `go-list` | `go list` | passes through |
| `go-get` | `go get` | passes through |
| `go-work` | `go work` | passes through |
| `staticcheck` | `staticcheck` | passes through |
| `golangci-lint` | `golangci-lint` | compresses |
| `govulncheck` | `govulncheck` | passes through |
| `cargo-build` | `cargo build` | compresses |
| `cargo-run` | `cargo run` | passes through |
| `cargo-check` | `cargo check` | passes through |
| `cargo-clippy` | `cargo clippy` | passes through |
| `cargo-fmt` | `cargo fmt` | passes through |
| `cargo-doc` | `cargo doc` | passes through |
| `cargo-add` | `cargo add` | passes through |
| `cargo-update` | `cargo update` | passes through |
| `cargo-tree` | `cargo tree` | passes through |
| `cargo-audit` | `cargo audit` | passes through |
| `cargo-outdated` | `cargo outdated` | passes through |
| `pip-list` | `pip list` | passes through |
| `pip-show` | `pip show` | passes through |
| `pip-freeze` | `pip freeze` | passes through |
| `pip-check` | `pip check` | passes through |
| `uv-pip` | `uv pip <read or install form>` | passes through |
| `uv-run` | `uv run <command>` | passes through |
| `ruff` | `ruff` | compresses |
| `mypy` | `mypy` | compresses |
| `black` | `black` | passes through |
| `isort` | `isort` | passes through |
| `pyright` | `pyright` | passes through |
| `bandit` | `bandit` | passes through |
| `poetry` | `poetry` | passes through |
| `pdm` | `pdm` | passes through |
| `make` | `make <target>` | passes through |
| `cmake` | `cmake` | passes through |
| `ninja` | `ninja` | passes through |
| `bazel` | `bazel` | passes through |
| `just` | `just` | passes through |
| `task` | `task` | passes through |
| `nx` | `nx` | passes through |
| `lerna` | `lerna` | passes through |
| `changesets` | `changesets` | passes through |
| `mkdir` | `mkdir` | passes through |
| `touch` | `touch` | passes through |
| `cp` | `cp` | passes through |
| `mv` | `mv` | passes through |
| `rm` | `rm` | passes through |
| `ast-grep` | `ast-grep` | passes through |
| `dotnet-build` | `dotnet build` | compresses |
| `dotnet-test` | `dotnet test` | passes through |
| `dotnet-run` | `dotnet run` | passes through |
| `rspec` | `rspec` | passes through |
| `rubocop` | `rubocop` | passes through |
| `mise` | `mise` | passes through |
| `asdf` | `asdf` | passes through |
| `direnv` | `direnv` | passes through |
| `psql` | `psql` | passes through |
| `gcloud` | `gcloud` | passes through |
| `journalctl` | `journalctl` | passes through |
| `systemctl` | `systemctl` | passes through |
| `kubectl-top` | `kubectl top` | passes through |
| `xcodebuild` | `xcodebuild` | passes through |
| `pod` | `pod` | passes through |
| `expo` | `expo` | passes through |
| `react-native` | `react-native` | passes through |
| `adb` | `adb` | passes through |
| `pip-install` | `pip install` (install form) | passes through |
| `mix` | `mix` | passes through |
| `swift` | `swift` | passes through |
| `sbt` | `sbt` | passes through |

Container exec (`docker exec`, `docker compose exec`, `docker-compose exec`) is wrapped when its inner command is an allowlisted family above (`phpunit`, `npm-test`, `artisan`, ...); the recorded family is the inner one. Accepted flags before the service or container: `-T`, `--no-TTY`, `-i`, `-u`/`--user`, `-w`/`--workdir`, `-e`/`--env KEY=VAL`. `-t`, `-it` and any other flag (`-d`, `--index`, `--privileged`) leave the command unwrapped, because a TTY changes the output the compressor reads. Nested docker is not wrapped.

Any other command, including `npm test`, is not wrapped. The shell still parses the wrapped command, so the words after the
family expand as they would have.

**Compound shapes (2026-10-21).** `internal/execwrap/policy_compound.go` also
accepts the shapes agents use most, matched word by word, and wraps only the
allowlisted command inside them; the rest is kept as written:

| shape | rewritten to | exit code |
|---|---|---|
| `cd <dir> && <cmd>` | `cd <dir> && '<bin>' compact -- <cmd>` | cd's on failure, else the command's |
| `<cmd> \| tail -N` (also `head`, `-n N`, `-nN`, N up to 5 digits) | `'<bin>' compact -- <cmd> \| tail -N` | the pipe's last command, as before |
| `<cmd> 2>&1 \| tail -N` | `'<bin>' compact -- <cmd> 2>&1 \| tail -N` | same |
| `cd <dir> && <cmd> 2>&1 \| tail -N` | both of the above | same |

`<dir>` passes the same safe-byte gate as the command (no `~`, `$`, quote, glob)
and cannot start with `-`. A deny or ask rule that matches the whole command,
the inner command, `cd <dir>` or the `tail`/`head` part stops the rewrite. Under
`| head -N` the wrapped command runs to its end before the output is cut, where
the bare command could have been stopped early by the closed pipe.

**Chains (2026-10-21).** `internal/execwrap/policy_chain.go` splits a command
at each `;` and `&&` and decides every segment alone, as a plain command or one
of the shapes above. Only allowlisted segments are wrapped; every other segment
and every separator stays byte-identical:

| shape | rewritten to | exit code |
|---|---|---|
| `gofmt -l x; go test ./pkg` | `gofmt -l x; '<bin>' compact -- go test ./pkg` | the last segment's, as before |
| `go build ./... && go test ./...` | each segment wrapped | `&&` short-circuits on the first failure, as before |
| `git diff; go test ./... 2>&1 \| tail -15` | both wrapped, the pipe kept | same |

The session tag goes on each wrapped segment once. A deny or ask rule that
matches the whole command or any segment (wrapped or kept, and the `cd` or
`tail` part of a shape) stops the rewrite. A command that already holds
`downshift compact` is never wrapped again.

Left unwrapped on purpose, and a chain is refused whole when any segment holds
one: `||`, a lone `&`, an empty segment (`;;`, a trailing `;`), a pipe into
anything but `tail`/`head -N` (`git diff | grep '^[+-]'`, `tee`, a second
pipe), `tail -f`, `tail -n +N`, file redirects (`>`, `>>`, `1>&2`), heredocs,
subshells, a pipe with no spaces around `|`, and anything with an expansion, a
glob or a quote (a quoted `;` is never split). Each of these can change what
the command does or which exit code the agent sees in ways a word match cannot
prove safe.

| harness | shell rewrite | status | limit |
|---|---|---|---|
| claude-code | `PreToolUse` on `Bash` → `updatedInput.command`, every sibling field kept, no `permissionDecision` | Wired, on by default | The plugin's `hooks.json` carries the `Bash` PreToolUse entry running `downshift claude-code`. Verified live on 2026-10-20 (see `docs/internal/tasks/2026-10-20-next-steps`). If Claude Code ever ignores `updatedInput` without a `permissionDecision`, the command runs unchanged: nothing breaks, nothing is saved. Permissions: no allow rule is installed or required. The rewrite is skipped when a deny or ask rule in the managed, user (`CLAUDE_CONFIG_DIR` honoured), project or local settings matches the **original** command (as written and normalised); an unreadable or invalid settings file also means no rewrite. The hook emits no `permissionDecision`, so Claude Code's own permission flow, including any approval prompt, applies to the rewritten command as it would to the original. Never add a broad `Bash(downshift compact:*)`. |
| codex | `PreToolUse` on `Bash` (also reported for `exec_command`) → `permissionDecision: "allow"` plus `updatedInput.command`, every sibling field kept | Wired, Unverified live | `downshift setup` writes a `^Bash$` entry running `downshift codex` next to the `Agent` one. Codex honors `updatedInput` only together with `permissionDecision: "allow"`, so a rewritten call is auto-approved by the hook; an unchanged call emits `{}`. Codex has no deny or ask rules the adapter can read, so `ShellBlocked` is always false and only the shared policy (allowlist, no metacharacters, kill switches) limits the rewrite. Unverified live until a Codex session runs a wrapped `go test` (the hook must be trusted in `/hooks`). |
| cursor | `preToolUse` on `Shell` → `updated_input.command`, every sibling field kept, no `permission` field | Wired | Unverified live; Cursor ignored `updated_input` for Task on some plans, Shell needs its own proof. `downshift setup` writes the `Shell` entry in `hooks.json`. No deny/ask rule is read (Cursor has no readable rule file), so the rewrite is not skipped by rules. Advise fallback: `beforeShellExecution` (matcher `*`) answers `allow` with `agent_message` naming the `downshift compact` form for an allowlisted family (`CanAdviseShellWrap`). It never denies and cannot rewrite. Unverified live: that Cursor shows `agent_message` to the agent on allow. |
| opencode | not wired | Unverified | Same. |
| antigravity | not wired | Unverified | Same. |
| kirocrew | not wired | No rewrite channel | Policy-mode only (allow or block), so it can never rewrite a command. |
| grok | none | No hook | Config only. |

`downshift compact -- <argv>` works the same from any shell, harness or not.

**Re-check on a live session (C14).** A unit test cannot show that Claude Code applies `updatedInput.command` when the hook returns no `permissionDecision`. To check it after a Claude Code upgrade:

1. Make sure the Downshift plugin is installed and no deny or ask rule matches `go test`. No allow rule is needed, and none should be added for `downshift compact`.
2. Start a new Claude Code session in a Go repository and ask it to run `go test ./...` (a repository with a dozen or more packages, all passing).
3. Look at the tool call Claude Code shows. The command must read `'<path>/downshift' compact -- go test ./...`, and a passing result must end with a `[downshift compact: compacted N to M bytes]` line.
4. Run `downshift context metrics`: `exec_metrics.commands` must have grown. With `DOWNSHIFT_EXEC=0` in the environment the command must run as typed.

If the command runs as typed with the hook on, Claude Code ignores the rewrite without a decision: nothing breaks and nothing is saved, and this feature should stay documented as not working there.

## Revalidation

Every TBD cell must be re-checked per harness release: hook contracts
evolve upstream and a green cell can regress silently. Failing proof,
the tool fails open and the spawn runs unchanged (README.md:755).

A 2026-10-09 sample of Codex `spawn_agent` inputs recorded key sets only
(`agent_type`, `message`, `task_name`, and sometimes `fork_turns`). It did
not include hook stdin, `transcript_path`, a model field, or executor
acknowledgement, so the Codex row above is not refreshed. See
[closeout-recheck-2026-10-09.md](../evidence/closeout-recheck-2026-10-09.md).

## Sources (README/docs only, no vendor claims)

- Claude paid: emission only until P1.3 (docs/evidence/rewrite-honored-protocol.md); plan limits: README § Plan compatibility
- Cursor free/legacy discard: README.md:734-735, README.md:738, README.md:1012-1013
- Codex flags + honored: README.md:744, README.md:746; evidence 2026-10-02: docs/evidence/codex-rewrite-honored-2026-10-02.md; evolving schema: README.md:373
- KiroCrew adapter capability and integration limits: [native hook setup](../guides/install.md#kirocrew-policy-mode-through-the-native-kiro-cli-hook), [routing guia](../guides/kirocrew-routing-guide.md), `internal/adapters/kirocrew/kirocrew.go`. A hook invocation counter or direct adapter smoke test does not prove live enforcement or the served child model.
- KiroCrew harness detection and `SessionEnv: KIRO_SESSION_ID` (PR #559, merged 2026-10-25): [kirocrew-harness-detection.md](https://github.com/tiagovilasboas/downshift-labs/blob/main/reverse/kirocrew/SPIKE.md), `internal/core/capabilities.go` (`KiroCrewCaps`), `cmd/downshift/session.go` (`envHarness`). The env var lets `envHarness()` attribute `downshift compactor`/`exec` events to `kirocrew`; it is not a hook-payload session id, so the `downshift session` tree still cannot group or pause KiroCrew. The spike lists the model set reported by `kiro-cli chat --list-models` (with `rate_multiplier`); catalog membership for those ids (T4), `SessionFromCatalog` (T5), `PayloadMarkers` (T2) and `AgentTypeHints` (T7) are open follow-ups in that spike, not shipped, so they stay documented as the matrix shows: `SessionFromCatalog` no, no payload markers, no type hints.
- Vendor hook docs vs adapters (search summaries, 2026-10-11): [vendor-hook-docs](../evidence/vendor-hook-docs-2026-10-11.md)
- Antigravity observed child, dated build and evidence limits: [native evidence](../evidence/antigravity-executor-ack.md) and [final QA](../evidence/final-gap-qa.md)
- Grok config-only: README.md:520-531, README.md:691, README.md:1236
- Emission vs honored: docs/reference/session-models.md:17; allowlist: docs/reference/session-models.md:34
