# Harness x plan x rewrite-honored matrix

> **Status:** Claude Code stable; Codex and Cursor functional; OpenCode beta; Antigravity and KiroCrew experimental.

Downshift by Tiago Vilas Boas
https://github.com/tiagovilasboas/downshift-ai

## Status per harness

Each harness carries exactly one of four status words (owner decision 2026-10-17; Beta added 2026-10-18); the table below it keeps the per-plan facts.

- **Completed:** rewrite observed honored on a real child, usage and child end recorded, plugin and session policy.
- **Functional:** the hook emits the rewrite on a real spawn and passes the shared conformance suite; observation is partial (honored is inferred, no child usage).
- **Beta:** first-class integration under active development — adapter, route wire and shared conformance in the repository and green; the harness-side plugin and the first live observation are still ahead, so no live spawn is observed yet and there is no production claim (owner decision 2026-10-18).
- **Experimental (proof of concept):** policy mode, one observed child on one build, or a rewrite emitted whose application is unconfirmed or silently discarded on some plans (owner decision 2026-11-07, Cursor); shipped, but no production claim.

`cmd/downshift/conformance_test.go` drives the same 8 routing scenarios against every registered hook, so each harness below, whatever its status, computes every feature (layering rule); the status says how much has been verified on the real harness, not how much code it runs.

| harness | status | why | evidence |
|---|---|---|---|
| claude-code | Completed | Rewrite observed honored (2026-10-06, API-reported child model); usage tokens read from the child transcript at `SubagentStop`; child end recorded; plugin and session policy ship. Failure payload is still GAP-CHILD-FAILURE (see "Child end" below). | [rewrite honored](../evidence/claude-code-rewrite-honored-2026-10-06.md), [billing audit](../evidence/billing-gap-audit.md), [routing A/B](../evidence/subagent-routing-ab-2026-10-15.md), [reconciliation](../evidence/claude-execution-workflow-reconciliation-2026-11-07.md) |
| codex | Functional | Rewrite emitted on a real spawn; honored is inferred from the next spawn, not observed; child end parser unverified, no usage source. | [codex rewrite](../evidence/codex-rewrite-honored-2026-10-02.md) |
| cursor | Experimental | Rewrite emitted on a real spawn; the child executor model is absent from the transcript, so honored is unconfirmed (discarded on Free and legacy Pro); end parser unverified, no usage; native quota collector blocked on Cursor 3.24.9. | [cursor rewrite](../evidence/cursor-rewrite-honored-2026-10-10.md), [gap diagnostic](https://github.com/tiagovilasboas/downshift-labs/blob/main/reverse/cursor/findings/F1-cursor-gap-diagnostic.md) |
| kirocrew | Experimental | Policy mode: block and retry observed once on Kiro CLI 2.29.0, no in-place rewrite; child end via postToolUse (unverified: needs real payload). | [native hook](https://github.com/tiagovilasboas/downshift-labs/blob/main/reverse/kirocrew/findings/F5-kirocrew-native-hook.md) |
| antigravity | Experimental | One observed child on desktop 2.21.1 (requested `pro`, executed `flash_lite`); the `overwrite` envelope is documented by the vendor hooks page (shallow-merged into `toolCall.args`), and the `run_command` shell rewrite built on it is documented, not live-verified; no child end recorded; quota mapping unresolved. | [executor ack](../evidence/antigravity-executor-ack.md), [runtime source](https://github.com/tiagovilasboas/downshift-labs/blob/main/reverse/antigravity/findings/F1-antigravity-runtime-source.md) |
| opencode | Beta | Integration through a bridge plugin that `downshift setup` installs at `~/.config/opencode/plugins/downshift/` (`index.ts` plus `models.ts`, `shell_sessions.ts`, `shell_trust.ts` and `wire.ts`). This reopens the plugin surface removed with the mods on 2026-10-29, at the owner's request for full OpenCode support. Live-verified on OpenCode 2.2.5: route to a smaller model, honored model chain, shell rewrite, child end and per-child token usage. Not live-verified yet: the retry fallback after a provider failure, and a `go test` rewrite through the plugin. | [adapter tests](../../internal/adapters/opencode/), [conformance](../../cmd/downshift/conformance_test.go) |
| grok | Config only | No hook rewrite; reasoning is pinned in `config.toml`. | [install.md](../guides/install.md#grok-cli-config-not-hook) |
| claude-desktop-chat | Experimental | Chat spawns no model-carrying subagent and has no shell tool, so model_rewrite, compaction and mid_session_routing are none; the read-only MCP tools (providers, status, playbook, apply, plan) answer over stdio, while delegate/collect/cancel/diff/send/steer/release fail with ErrNoVerifiedMode when the worker mode is off. Harnesses with an MCP client get the worker tools; harnesses without one see the computed state in `downshift doctor` and `downshift report`. Not a hooked harness: it has no row in the per-plan, child-end or discovery tables. | [F2-chat-mcp](https://github.com/tiagovilasboas/downshift-labs/blob/main/reverse/claude-desktop/findings/F2-chat-mcp.md) |
| claude-desktop-cowork | Experimental | The Claude Desktop local agent runs in a Linux arm64 VM that shares only granted folders over virtiofs; host hooks, the plugin, ~/.claude and the macOS binary never reach the guest, so model_rewrite, compaction, token_counts, mid_session_routing and savings_estimation are none and MCP reachability is unverified. Not a hooked harness: it has no row in the per-plan, child-end or discovery tables. | [F3-cowork-vm](https://github.com/tiagovilasboas/downshift-labs/blob/main/reverse/claude-desktop/findings/F3-cowork-vm.md) |

Claude Code, 12 of 14 inferred honored (2026-10-29, counts only, no prompts):
one miss was a genuine ignored rewrite, a Haiku rewrite whose linked `resolved`
record, 93 ms later, reported Opus with `rewrite_honored: false`. It matches the
`CLAUDE_CODE_SUBAGENT_MODEL_FORCE` override described in the CHANGELOG (doctor now
fails on it); routing is unchanged. The other had no `resolved`, `child_ended` or
later spawn in its session, so it is unobserved, not refuted. Two more shifts
without a session id are outside the count. `doctor` now reports these apart:
`seen: 12 of 14 rewrites (1 refuted: ...; 1 not observed: ...)`.

"Rewrite honored" means the harness executor applied the hook's model
rewrite to the child subagent, not just that downshift emitted one.
`outcome: "rewrite_emitted"` proves emission only (docs/reference/session-models.md:17).

| harness | plan / build | rewrite honored? | evidence / limit |
|---|---|---|---|
| claude-code | paid: Pro / Max / Teams / API | Yes, 2026-10-06 (two spawns; API-reported model in the child transcript) | `updatedInput.model` must be a family name (`haiku`/`sonnet`/`opus`/`fable`); a full id is rejected and blocks the spawn. The catalog `native_name` supplies it. Write-up: `docs/evidence/claude-code-rewrite-honored-2026-10-06.md`. There is no built-in agent-type table (owner decision O8): the orchestrator chooses the model per subagent, and the `downshift-orchestrator` playbook carries the defaults. Setup does not write `CLAUDE_CODE_SUBAGENT_MODEL_FORCE`, so a per-call model from the orchestrator is honoured. The payload carries no session list; without one (payload, discovery or `session-models.json`) the catalog `session_candidates` below the current model stand in for it (`SessionFromCatalog`, downshift only, never above the current model); a list is needed only to allow upshifts. Effort parameter (`effort` on Agent call, v2.1.292+) rewriting in PreToolUse is unobserved in settings hooks without engine mods (`CanApplyEffort: no`); `CLAUDE_CODE_EFFORT_LEVEL` in the environment overrides any per-call effort. Detailed reconciliation: [claude-execution-workflow-reconciliation-2026-11-07.md](../evidence/claude-execution-workflow-reconciliation-2026-11-07.md). |
| claude-code | nested spawn (a subagent spawning a subagent), Claude Code 2.1.319 | Not applicable: verified 2026-11-07, a subagent has no Agent tool, even when a custom agent declares it | Two live sessions; the hook log holds only the main session's `Agent` call. A subagent's other tool calls reach the hooks with `agent_id` and `agent_type`, which `isSubagentCall` already reads. Other builds and agent teams unverified. `docs/evidence/claude-code-nested-spawn-2026-11-07.md` |
| claude-code | workflow agents (spawns from Claude Code Workflow scripts) | No: bypasses PreToolUse (GAP-WORKFLOW-AGENTS, 2026-10-11) | Spawns from Workflow scripts bypass `PreToolUse`; only `SubagentStart` fires, with no task prompt and no model field. `updatedInput` is ignored. Downshift cannot route them; set the model on the workflow `agent({model: ...})` call. Write-up: `docs/evidence/workflow-agents-bypass-pretooluse-2026-10-11.md`. |
| claude-code / anthropic-platform | Managed Agents (Anthropic cloud multiagent orchestration, beta) | Not applicable: distinct hosted API surface | Anthropic Managed Agents (`/v1/agents`, `/v1/sessions`, header `managed-agents-2026-04-01`, `multiagent_20261001`) runs server-side in hosted cloud sandboxes. Local Claude Code `PreToolUse` hooks do not execute on Managed Agents platform threads (subagents, dynamic workflows, advisor). Model routing is declared via `predefined_agents` or session overrides, not local client hooks. |
| claude-code | Free | No: no real subagents | Sandboxed, no egress; subagents need paid (README.md:728) |
| cursor | Free | No: silently discarded | `updated_input.model` ignored, known issue (README.md:734) |
| cursor | Pro legacy (request-based) | No: silently dropped | Task model takes only `"fast"` (README.md:735) |
| cursor | Pro / Ultra usage-based | Unconfirmed | Actual child-model proof remains P1.6. Separately, Cursor 3.24.9 blocks the external native quota collector before transport; see [diagnostic](https://github.com/tiagovilasboas/downshift-labs/blob/main/reverse/cursor/findings/F1-cursor-gap-diagnostic.md). |
| codex | multi_agent_v2 + hooks on | Yes, inferred (2026-10-02) | Inferred, not observed: same-session follow-up: spawn requested `gpt-5.6-terra` after hook emitted Terra (`docs/evidence/codex-rewrite-honored-2026-10-02.md`). Payload >1MB still fail-open. Schema still evolving upstream. |
| kirocrew | Kiro CLI 2.29 native spec hook; MCP `spawn_run` with explicit `task` + `model`, or `spawn_run` with `tasks[]` (each entry `{task, model, reasoning_effort}`) | Policy/block + retry observed 2026-10-10; no in-place rewrite | Persistent `agent.kiro_hooks.preToolUse` + executable wrapper; no `updated_input` channel. Native `use_subagent` without child model is outside coverage. `hooks.json` tool-call hooks are informational and do not enforce blocks. [Native evidence](https://github.com/tiagovilasboas/downshift-labs/blob/main/reverse/kirocrew/findings/F5-kirocrew-native-hook.md); [setup and limits](../guides/install.md#kirocrew-policy-mode-through-the-native-kiro-cli-hook). `spawn_run` `tasks[]`: each entry is routed on its own, with one decision and one telemetry row per entry; a block names every entry to respawn and the whole call is blocked (KiroCrew cannot rewrite one entry). Batch rows carry no top-level model, so their telemetry `FromModel` is `unknown`. A top-level `spawn_run` without `tasks` routes as before. `spawn_sub_agents` is **not routed** (passes through unchanged). Shape verified in the KiroCrew 2.29 backend source (`validation.py` `SPAWN_SUB_AGENTS_SCHEMA`, `mcp_tools/spawn.py`), not a captured payload: `agents: [{agent_or_mode, prompt}]` with no per-entry model; the handler forwards only task, agent and context flags, so the child model is set by the named agent. Downshift cannot route it. The postToolUse matcher uses glob `*spawn_run*` to match both the bare tool name and the MCP-qualified `mcp_kirocrew_core_spawn_run`. Table tests: `internal/adapters/kirocrew/batch_test.go`. |
| antigravity | desktop 2.21.1, `invoke_subagent` | Yes, observed 2026-10-09 | One exact child: original `pro`, executed `flash_lite`, native executor/generation model `gemini-3.5-flash-lite`. [Evidence and QA](../evidence/antigravity-executor-ack.md). Manual read-only, build-specific sensor; quota/picker/alias mapping remains unresolved. |
| opencode | 2.2.5 with the bridge plugin, zen models (no published list price) | Yes, observed on OpenCode 2.2.5 | Through the bridge plugin that `downshift setup` installs. The retry fallback after a provider failure is not live-verified yet; the bridge validates a retry call like a route call and undoes the route only on attempt 2, the first retry opportunity. A session model id that contains a slash (`openrouter/deepseek/deepseek-v4.1-flash`) is kept whole when matched. The route answer carries the target model; the observation side reports a resolved model only from an arrival-ordered switch chain (`switch_attempt` ok, then `model_selected`, then the first `child_context`) compared against the decision target. Child usage comes from OpenCode's step token report (summed once per assistant message, cache reads kept apart); no cost is recorded because no price is published for zen models. |
| grok | any (single model grok-4.6) | No hook rewrite: config only | PreToolUse is allow/deny; pin `reasoning_effort` in config.toml (README.md:520-531) |

Packaging note: the plugin (`.claude-plugin/`, `hooks/hooks.json`, `agents/`)
loads only on Claude Code. Every other harness gets the same hooks through
`downshift setup` or the manual sections of
[install.md](../guides/install.md); the skills under `skills/` are plain
SKILL.md files any harness can read. `downshift setup` also copies the
read-only `downshift-audit` skill into `~/.claude/skills` and `~/.codex/skills`
(the `SkillsDir` field of the setup target); a harness without that field gets
no skill from setup and can read `skills/` directly. Setup skips the copy when
the plugin is active, and `--undo` removes only files that still carry the
ownership phrase.

"Inferred" means a later spawn in the same session asked for the model the
hook had written (`rewrite_honored_inferred` in `stats --export`). It is
correlation, not a read of the child's actual model. Only applied rewrites
(outcome `rewrite_emitted`, no guardrail correction) count; allow, blocked
and held events never do.

Per-harness measurement (what is observed, what blocks per-token figures, which catalog prices apply, and catalog anomalies):
[value-per-harness.md](value-per-harness.md).

## Capability grid (`HarnessCapabilities`)

One row per capability field in `internal/core/capabilities.go`, one column per
harness in `core.AllHarnessCaps()`. A cell is `yes`, `no` or `partial`, with an
optional note after it (`yes (note)`). For switches, `yes` means true; for
declared data (env names, events, maps, caps), `yes` means a value is declared and
`no` means empty, which is "unverified or none", not proof of absence.
`internal/limits/capability_grid_test.go` fails when a cell differs from the code,
so flip the code and this table together. `NativeUsageSource` and
`MainToolCategories` have no row: they only qualify `NativeSessionEnv` and
`MainToolEvent`. Grok is not in `AllHarnessCaps()` and has no column.

| capability | claude-code | cursor | codex | kiro | kirocrew | antigravity | opencode | meaning |
|---|---|---|---|---|---|---|---|---|
| `CanRewriteModel` | yes | yes | yes | yes | no | yes | yes | The hook can replace the subagent model. |
| `CanApplyEffort` | no | no | yes | no | no | no | no | The hook can set reasoning effort. |
| `RewritePreservesInput` | yes | yes | yes | yes | no | yes | no | The rewrite echoes every sibling field of the spawn input. |
| `StrictModelName` | yes | no | no | no | no | no | no | The harness rejects model names outside its own aliases. |
| `ModelOverrideEnv` | yes | no | no | no | no | no | no | An env var is known to make the harness ignore a rewrite. |
| `ModelForceEnv` | yes | no | no | no | no | no | no | An env var forces one model for every subagent. |
| `NativeSessionEnv` | no | yes | no | no | no | no | no | Env var naming the native session or usage export. |
| `TranscriptQuota` | no | no | yes | no | no | no | no | Quota can be collected from an explicit transcript. |
| `SingleModelReasoning` | no | no | no | no | no | no | no | One model, only reasoning effort is tunable. |
| `SessionFromCatalog` | yes | no | no | no | no | no | no | Unknown session built from catalog `session_candidates`. |
| `CanShowChildLine` | yes | no | no | no | no | no | no | The hook protocol can show the child-end line. |
| `CanReturnNote` | yes | no | no | no | no | no | no | The PreToolUse answer can carry a note the model reads. |
| `SessionEnv` | yes | no | no | no | yes | no | no | Env var holding the current session id. |
| `ShellEnv` | no | no | yes | no | no | no | yes | Env var the harness sets in agent shells. |
| `AgentEnv` | no | no | no | no | no | no | yes | Env vars whose presence means an agent runs the command. |
| `ChildEndEvent` | yes | yes | yes | yes | yes | no | yes | A hook event carries the end of a child spawn. |
| `UsageSource` | yes | no | no | no | no | no | yes | Child token usage is read (payload or transcript). |
| `HookEntries` | yes | no | no | no | no | no | no | Known spawn-time entry points are listed. |
| `PayloadMarkers` | no | yes | no | no | no | no | no | Payload keys only this harness sends. |
| `CanRewriteShellCommand` | yes | yes | yes | yes | no | yes | yes | The hook can rewrite the shell command in place. |
| `RewriteImpliesApproval` | no | no | yes | no | no | no | no | The shell rewrite answer also approves the command, so only one read-only command is rewritten. |
| `UnknownUserRules` | no | yes | yes | yes | no | yes | no | The user's deny/ask rules cannot be read, so only one read-only command is rewritten (a rule on the original text could miss the wrapped form). |
| `ShellDialect` | yes | yes | yes | yes | no | yes | yes | The shell is declared POSIX (`posix`). Only then are the compound (a `tail -N` pipe, a `cd d &&` prefix) and chain (`&&`, `;`) shapes rewritten; a harness with an empty or other dialect keeps them as written, and one plain command is wrapped either way (KiroCrew declares none, so its advise path skips those shapes too). |
| `CanAdviseShellWrap` | no | yes | no | no | yes | no | no | The hook can advise a wrapped shell command (exit 2). |
| `AgentTypeHints` | yes | no | no | no | no | no | no | Subagent type names map to a tier. |
| `SquadAgentPrefix` | yes | no | no | no | no | no | no | Squad agents arrive with a type prefix. |
| `MainToolEvent` | yes | no | no | no | no | no | no | An event reports main-session tool calls. |
| `MaxParallel` | no | no | no | no | no | no | no | A parallel subagent cap is known. |
| `WorkerAgent` | yes | no | yes | no | no | no | no | A worker agent CLI runs on this harness. |
| `WorkerReadOnlyVerified` | yes | no | no | no | no | no | no | A read-only worker was verified unable to write. |
| `SessionWideTranscript` | yes (nested spawn, 2026-11-06, Claude Code 2.1.319: transcript_path verified, nested spawn payload unverified) | no | no | no | no | no | no | A hook fired inside a subagent still gets the main session's `transcript_path`, so a nested spawn (a subagent spawning a subagent) cannot read its parent's model from it. With `agent_id` set the baseline is unknown and the spawn is held; no parent model is recorded. Other harnesses: nesting not assessed. See the note below. |

**Nested spawns (Claude Code 2.1.319, 2026-11-06).** Verified live: a `PreToolUse` hook fired inside a subagent (a Bash call) carries `agent_id` and `agent_type`, and its `session_id` and `transcript_path` equal the main session's; the subagent's own transcript appears only as `agent_transcript_path` on `SubagentStop`. Verified live: a subagent has no `Agent` tool, even when its definition lists one (two runs), so a nested `Agent` spawn could not be captured and its `PreToolUse` payload is **unverified**; routes that do nest (teammates, workflow agents) are not captured here either (workflow agents never reach `PreToolUse`, see the evidence note of 2026-10-11). The rule follows the documented hook input: a spawn with `agent_id` set and no model named by the call, the agent definition or the environment is held with an unknown baseline, and the main model is not recorded as its `parent_model`; a model the call names, or a parent model the mod reports, still routes as usual. Test: `TestHookE2E_ClaudeCodeNestedSpawn`. Telemetry keys decisions by the main session's hash, so a nested spawn counts under the main session, once. Detailed write-up and reconciliation: `docs/evidence/claude-code-nested-spawn-2026-11-07.md` and `docs/evidence/claude-execution-workflow-reconciliation-2026-11-07.md`.

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
  `HELD_UNCONFIDENT_UNKNOWN_MID` and never routed. A declared policy profile (e.g.
  `explore` on the small tier, proposed by `downshift policy init` from PR #18)
  matches the agent type case-insensitively (`Explore` matches `explore`) and
  routes the spawn to the small tier (such as Haiku), bounding the orchestrator's
  model choice on every such spawn as explicit user policy, even when the prompt is
  classified as complex. If the caller pins a model above its agent default,
  Downshift leaves it as pinned and emits an advisory note to the orchestrator via
  `additionalContext` (`CanReturnNote: true`).
- **Claude Code Workflows vs Anthropic Managed Agents.** Two separate orchestration surfaces exist across the vendor documentation:
  1. *Claude Code `Workflow` scripts (GAP-WORKFLOW-AGENTS):* Local client scripts spawning subagents via `agent()` bypass `PreToolUse` entirely (only `SubagentStart` fires, without task text or model, and `updatedInput` is ignored; write-up: `docs/evidence/workflow-agents-bypass-pretooluse-2026-10-11.md`). Downshift cannot classify or rewrite them; users must declare `model` directly on the workflow call. Subagents are denied the `Workflow` tool by default built-in tool filtering.
  2. *Anthropic Managed Agents (`multiagent_20261001`, beta):* A distinct hosted server-side API (`/v1/agents`, `/v1/sessions`, header `managed-agents-2026-04-01`) coordinating subagents, dynamic workflows, and mid-turn advisor models (`anthropic.advisor`). Runs entirely server-side in hosted cloud sandboxes. Local Claude Code `PreToolUse` hooks do not execute on Managed Agents platform threads. Model selection on Managed Agents is declared via `predefined_agents`, inline defaults, or session overrides, not local client hooks. Comprehensive reconciliation: `docs/evidence/claude-execution-workflow-reconciliation-2026-11-07.md`.
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
- **Upshift ranking, all harnesses.** Downshift never raises a tier on its own: the only `UPSHIFT` verdict is an operator tier-floor lift.
  Serving it, Downshift ranks the session ids by catalog tier, not list position, never goes below the current model's tier, and lands on
  the least model at the floored tier; when nothing reaches the floored tier it takes the strongest id at or above the current model's tier,
  still never below the floor. The legacy `explicit_upshift` opt-in that once let a lift reach an `explicit_only` model was removed 2026-10-31;
  an `explicit_only` id is never selected in any direction. The rule and its fallback are in [session-models.md](session-models.md).
- **Cost linkage.** Every decision records `tool_use_hash`. PostToolUse
  repeats the id, so `resolved`, `child_ended` and, through the agent id,
  `usage` attach to the exact decision even with parallel spawns, including
  held and allow decisions (no `rewrite_honored` claim for those), and
  paused decisions (`outcome: baseline`): before
  2026-10-19 the linker treated `baseline` as a cost record and skipped it, so
  no baseline spawn had an end record. Lines written before this field
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

Parent model on the decision event: the shared runner writes `parent_model` and
`parent_model_source` on every decision whose adapter exposes the optional
`ParentModelIdentifier` method, also when the call names its own model. Data
only; routing never reads it. Absent when unknown.

| harness | `parent_model` | `parent_model_source` |
|---|---|---|
| claude-code | yes | `mod` (agent.spawn), else `transcript` (assistant line of this call, bounded read) |
| codex | yes | `payload` (`model`), else `transcript` (rollout `turn_context`) |
| cursor | yes | `payload` (`model_id`, `model`), else `transcript` |
| kirocrew, antigravity, opencode, grok | no | the payload carries no parent model the adapter reads |

Consequences: Claude Code is the only harness where the catalog candidates are
safe (restored 2026-10-27 after PR #570 removed the inferred ladder and left
every Claude Code spawn unknown); every other harness needs a real list (hook payload, discovery cache or
the operator file). Quota stays `unknown` everywhere except through the Claude
Code statusline bridge and operator imports. Re-verify per release.

Claude Code quota routing needs the statusline, which the Claude Desktop app does not run; `downshift doctor` warns when there is no snapshot and the desktop app is installed.

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
| opencode | `child_done` (observation envelope the plugin forwards) | payload (`UsageSource: payload`): the bridge sums `session.step.ended` token counts per child and sends them on `child_done`. Nothing is priced. | Live-verified on OpenCode 2.2.5: the envelope, the honored chain, the end record and the usage record all linked to the decision. The honor reader reports a resolved model only from the arrival-ordered switch chain; a batch without it records nothing. |

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

## Agent default tier (`default_tier`)

Each squad agent (`agents/*.md`) declares `default_tier: small|mid|frontier` in its frontmatter; the files are
embedded in the binary (`internal/routing/agenttier`). A `downshift:<agent>` spawn that names **no model** runs on
that tier instead of inheriting the session model (verdict `agent_default`, reason code `AGENT_DEFAULT_TIER`).
It only moves down from the inherited model. A named model, a declared profile or rule, and the project tier floor
win; a `[tier=...]` hint still only moves down and never lands above the default. An unknown agent type, or one
outside the `downshift:` scope, behaves as before. Every harness computes it in the shared decider and records the
reason code; the harness column says who can write it.

The agent type is exposed by the adapter; the tier is resolved in the shared decider (`gatherInput`). The prefix of
the squad's type is data (`HarnessCapabilities.SquadAgentPrefix`).

| harness | agent type read | squad type | named-model fact | applies it | only computes |
|---|---|---|---|---|---|
| claude-code | `subagent_type` | `downshift:<agent>` | Task or payload model, definition pin, subagent env | yes (rewrites `model`) | |
| cursor | `subagent_type` | bare name | `tool_input.model` empty or `inherit` | yes (rewrites `model`) | |
| codex | `agent_type` | bare name | `tool_input.model` empty | yes (rewrites `model`) | |
| kirocrew, antigravity, opencode | none in the payload | n/a | n/a | | not reached: use the `[tier=small]` hint |

A harness whose protocol cannot write the model still gets the decision and the `AGENT_DEFAULT_TIER` reason code
from the shared decider (tested over `core.AllHarnessCaps()`). A harness that sends no agent type gets the benefit
only through the orchestrator's `[tier=small]` prompt hint and the MCP tools: `providers` and the `playbook` list
name each squad agent's default tier (agent and tier only, no model ids), so the orchestrator of any harness can choose
by it. `routing.agent_default_tier: false` in the policy turns the automatic step off.

### Pinned above the default

When a spawn names a model whose tier is above its agent's default (a `coder` pinned to frontier), Downshift
changes nothing and tells the orchestrator once: "downshift: coder defaults to mid; you pinned frontier. It runs as
pinned." The decider computes it for every harness and records the reason code `ABOVE_AGENT_DEFAULT` (no task
text). The note itself is returned only where `HarnessCapabilities.CanReturnNote` is true: the PreToolUse answer
can carry text the model reads.

| harness | note returned to the orchestrator | reason code |
|---|---|---|
| claude-code | yes (`additionalContext`, probed on 2.1.316) | yes |
| cursor, codex | no (not verified) | yes |
| kirocrew, antigravity, opencode | no | no: the payload carries no agent type |

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
hook payload: computed and logged, not groupable, not pausable. KiroCrew's `SessionEnv` does let a
command run from the harness (the `downshift compactor`/`exec` shell wrap)
attribute its exec event to `kirocrew` instead of `unknown`, through
`envHarness()` reading `KIRO_SESSION_ID`; the tree grouping still needs a
hook-payload id it does not have. The `/downshift` slash command is Claude
Code only. See [session-view.md](../guides/session-view.md).

## Main-session mechanical work (`MainToolEvent`, 2026-11-03)

`downshift report` counts the read, search and shell calls the main session
makes itself, which a small-tier subagent could take, and how many ran on a
frontier model. One row per harness, never summed. A record holds the tool
name, a category code (`read`, `search`, `shell`, `edit`, `other`), the hashed
session id and the harness (the session model's tier is resolved when the report runs, from the session's latest decision parent model, never per call); no command, path, pattern or prompt. A call with
an `agent_id` came from a subagent and is not recorded. The log is
`main-work.jsonl`, rotated like the event log.

The capability is data: `HarnessCapabilities.MainToolEvent` (the hook event that
reports each main-session tool call) and `MainToolCategories` (tool name to
category). An empty `MainToolEvent` means the row reads "unobserved", not 0.

| Harness | Observed | Evidence |
|---|---|---|
| claude-code | yes, for the tools its `PostToolUse` hook matches | `PostToolUse` carries `tool_name`; a top-level `agent_id` (with `agent_type`) marks a call made inside a subagent, as the Claude Code hooks reference documents for common hook input; the same `agent_id` field is what the SubagentStop hook reads here. Not yet confirmed from a live PostToolUse capture on this machine (the capture run was refused by the permission classifier), so a transcript path under a `subagents/` directory is also treated as a subagent call. Setup and the plugin wire the `Bash` and `Read|Grep|Glob` matchers, so read, search and shell calls are counted (an install made before 2026-11-03 needs `downshift setup` again for the read and search matcher). Read and search results are not measured by the context sensor. |
| cursor | no | no per-tool event wired (`preToolUse` is Task and Shell only, for routing) |
| codex | no | `PostToolUse` is wired for spawns only |
| kirocrew | no | `postToolUse` is wired for `spawn_run` only |
| antigravity | no | only `invoke_subagent` is hooked |
| opencode | no | the bridge forwards spawn intent, retries, shell commands and `child_done` only; no per-tool event |

Read result size (2026-11-06, #892, measure only): a `read` record also holds
`result_bytes_bucket`, the size class of the result (`<1K`, `1-4K`, `4-20K`,
`20-100K`, `>100K`; 1K = 1024 bytes), computed in `internal/mainwork` from a
byte length the adapter exposes. The exact byte count is stored too (`result_bytes`, a number); no content or path is, and
the hook emits nothing to the harness. Claude Code: yes, the length of the
string values in `tool_response` (Read's response shape is not documented; any
nesting is walked). Cursor, Codex, KiroCrew, Antigravity, OpenCode: no, they
send no per-tool event, so nothing is recorded. `downshift report` prints the
share of Read results and of Read bytes per class (`read_result_buckets` and
`read_result_bytes` in the JSON summary).

## Console or plain output per harness

A read-only command run by a person in a terminal opens the console; every
machine call stays plain ([cli.md](cli.md#read-only-commands-open-in-the-console)).
An agent's shell is usually not a terminal, so it gets plain text through the
TTY rule. When a harness runs commands in a pseudo-terminal, only a known
agent variable keeps it plain. The variables are data in
`HarnessCapabilities` (`ShellEnv`, `SessionEnv`, `AgentEnv`), declared only
when a source confirms them.

| Harness | Variable that keeps its agent shell plain | Gap |
|---|---|---|
| claude-code | `CLAUDE_CODE_SESSION_ID` | none known |
| codex | `CODEX_THREAD_ID` | none known |
| kirocrew | `KIRO_SESSION_ID` | none known |
| opencode | `OPENCODE_SESSION_ID`, `OPENCODE` | none known |
| cursor | none verified | if its agent terminal is a pseudo-terminal, a read-only command opens the console |
| antigravity | none verified | same as cursor |

Where a gap is listed, the worst case is a read-only screen (commands that
write never open the console). Set `DOWNSHIFT_PLAIN=1` in that harness's agent
environment, or pass `--plain`, to force plain text.

## Mods

Downshift bundles no Claude Code mods: the vendored ones were removed on 2026-10-29. A mod is code Claude Code runs in its own process, so it exists only where mods load; the other harnesses have no mod system.

## Agent triggers in always-loaded instructions (`setup`, 2026-11-04)

`downshift setup` writes the agent triggers (`downshift full power`, `downshift shift <squad>`) into the instructions each harness loads every session, so the phrase works without the `downshift-orchestrator` skill being opened. Details: [install.md](../guides/install.md#agent-triggers-every-harness-with-an-instructions-file).

| Harness | Where | Status |
|---|---|---|
| Claude Code | `~/.claude/CLAUDE.md`, marked block `downshift:agents` | Written by setup |
| Codex | `~/.codex/AGENTS.md`, after the compact steering | Written by setup; an `AGENTS.override.md` wins and hides it |
| Antigravity | `~/.gemini/AGENTS.md`, after the compact steering | Written by setup |
| OpenCode | `~/.config/opencode/AGENTS.md`, after the compact steering | Written by setup |
| KiroCrew | workspace steering file `downshift-compact.md` | Written by setup |
| Cursor | none: no global instructions file setup can write | Gap: copy the asset into a project rule or User Rules by hand |
| Claude Desktop Chat | none: MCP host only | Not applicable |

Unverified: a new session in each real harness picking up the text.

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

CCR (on by default; `DOWNSHIFT_CCR=0` turns it off; `"ccr": false` keeps only failed runs) stores the raw output of a compacted command and adds `downshift retrieve <id>`. Both are plain commands, so CCR works in every harness, hooked or not; the MCP tool `retrieve` is the same read. See [ccr.md](../guides/ccr.md).

On by default on Claude Code: the plugin registers the `Bash` PreToolUse hook. Set
`"exec": false` in the Downshift config or `DOWNSHIFT_EXEC=0` to turn it off. The decision is
shared (`internal/execwrap.Decide` plus `decide` in `internal/shellwrap`): the command must start with one of the families below, contain no
shell metacharacter (or match one of the compound shapes below, only on a harness whose `ShellDialect` is `posix`), not already start with `downshift`, and be rewritten to the
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
| `flutter-run` | `flutter run` (never wrapped: it runs the app until stopped) | passes through |
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
| `phpstan` | `phpstan`, `phpstan analyse`; `--generate-baseline`, `--watch`, `--fix`, `--pro` and other subcommands refused (clean run only) | compresses |
| `psalm` | `psalm` | passes through |
| `php-cs-fixer` | `php-cs-fixer` | passes through |
| `pint` | `pint` | passes through |
| `pest` | `pest` | passes through |
| `rector` | `rector` | passes through |
| `sail-test` | `sail test` | passes through |
| `sail-artisan` | `sail artisan <command>` | passes through |
| `vitest` | `vitest`, `npx vitest`, `pnpm exec vitest`, `yarn vitest`; not `vitest watch`, `vitest dev`, `--watch` or `-w` | compresses |
| `jest` | `jest`, `npx jest`, `pnpm exec jest`, `yarn jest`; not `--watch` or `--watchAll` | compresses |
| `playwright` | `playwright test`, `npx playwright test`, `pnpm exec playwright test`, `yarn playwright test` | compresses |
| `cypress` | `cypress run`, `npx cypress run`, `pnpm exec cypress run`, `yarn cypress run` | compresses |
| `tsc` | `tsc`, `npx tsc`, `pnpm exec tsc`, `yarn tsc`; not `--watch` or `-w` | compresses |
| `next-build` | `next build`, `npx next build`, `pnpm exec next build` | compresses |
| `vite-build` | `vite build` | passes through |
| `vite-preview` | `vite preview` (never wrapped: it serves until stopped) | passes through |
| `turbo-run` | `turbo run` | compresses |
| `npm-install` | `npm install` | compresses |
| `npm-test` | `npm test` | compresses |
| `npm-build` | `npm build` | passes through |
| `pnpm-test` | `pnpm test` | compresses |
| `pnpm-build` | `pnpm build` | passes through |
| `yarn-test` | `yarn test` | compresses |
| `yarn-build` | `yarn build` | passes through |
| `bun-test` | `bun test` | passes through |
| `bun-build` | `bun build` | passes through |
| `eslint` | `eslint`, `pnpm exec eslint`, `yarn eslint` | compresses |
| `prettier` | `prettier` | passes through |
| `biome` | `biome` | passes through |
| `mysql-query` | `mysql -e <one read statement>`, with connection and output options only (no `INTO`, no `EXPLAIN ANALYZE`, no second statement) | passes through |
| `redis-read` | `redis-cli <read-only command>` (and `valkey-cli`), never `--pipe` | passes through |
| `kubectl-get` | `kubectl get`; `-w`, `--watch`, `--watch-only` refused (the plain `NAME READY STATUS RESTARTS AGE` pod table only) | compresses |
| `kubectl-describe` | `kubectl describe` | passes through |
| `kubectl-logs` | `kubectl logs`; not `-f`/`--follow` | passes through |
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
| `aws-logs` | `aws logs`; not `tail --follow` | passes through |
| `docker-ps` | `docker ps` (the default table only; a row that is not a container row keeps the output raw) | compresses |
| `docker-logs` | `docker logs`; not `-f`/`--follow` | passes through |
| `docker-images` | `docker images` (the default table only) | compresses |
| `docker-build` | `docker build`; `--push`, `-o`/`--output`, `--secret`, `--ssh`, `--cache-to` and a URL context refused (classic builder transcript; any error, failure or warning line keeps it raw) | compresses |
| `argocd-list` | `argocd list` | passes through |
| `argocd-get` | `argocd get` | passes through |
| `argocd-diff` | `argocd diff` | passes through |
| `kustomize-build` | `kustomize build` | passes through |
| `az-pipelines` | `az pipelines <read form>` | passes through |
| `logcli` | `logcli`; not `query --tail`/`-t` | passes through |
| `sentry-cli` | `sentry-cli` | passes through |
| `grafana-cli` | `grafana-cli` | passes through |
| `datadog-ci` | `datadog-ci` | passes through |
| `git-status` | `git status` | compresses |
| `git-log` | `git log` | compresses |
| `git-diff` | `git diff` | compresses |
| `git-show` | `git show` | compresses |
| `git-branch` | `git branch` | passes through |
| `git-fetch` | `git fetch` (progress and hook noise cut; refs and errors kept) | compresses |
| `git-pull` | `git pull` (progress and hook noise cut; refs and errors kept) | compresses |
| `git-push` | `git push` (progress, hook banners and clean go test lines cut; refs, errors, failing tests and the hook summary kept) | compresses |
| `git-commit` | `git commit` (hook banners and clean go test lines cut; the commit line, errors, failing tests and the hook summary kept) | compresses |
| `git-add` | `git add` | passes through |
| `git-stash` | `git stash` (only `git stash list` output is recognised; every other stash form prints raw) | compresses |
| `git-remote` | `git remote` | passes through |
| `git-tag` | `git tag` | passes through |
| `git-config-read` | `git config --get`, `--get-all`, `--get-regexp`, `--list` (read flags only) | passes through |
| `git-other` | any other `git <subcommand>` not listed above | passes through |
| `gh-pr` | `gh pr view`, `gh pr checks` | compresses |
| `gh-run` | `gh run list`, `gh run view --log` | compresses |
| `gh-api` | `gh api` | passes through |
| `gh-issue` | `gh issue` | compresses |
| `gh-repo` | `gh repo`, `gh release view` | compresses |
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
| `make` | `make <target>` (a go test run in its output folds; a failed target cuts only the passing go tests, every other line is kept) | compresses |
| `cmake` | `cmake` | passes through |
| `ninja` | `ninja` | passes through |
| `bazel` | `bazel` | compresses |
| `just` | `just` | passes through |
| `task` | `task` | passes through |
| `nx` | `nx` | compresses |
| `lerna` | `lerna` | passes through |
| `changesets` | `changesets` | passes through |
| `mkdir` | `mkdir` | passes through |
| `touch` | `touch` | passes through |
| `cp` | `cp` | passes through |
| `mv` | `mv` | passes through |
| `rm` | `rm`, never recursive (`-r`, `-R` or `--recursive` in any spelling) | passes through |
| `ls` | `ls` with the flags `-a -A -l -1 -p -r -t -S` and at most one path; `-R`, `-s`, `-h`, `-C`, `--color` and other long options are refused | compresses |
| `find` | `find`; `-exec`, `-execdir`, `-ok`, `-okdir`, `-delete`, `-fprint`, `-fprint0`, `-fprintf`, `-fls`, `-ls`, `-printf`, `-print0` and `-X` are refused | compresses |
| `grep` | `grep` with a pattern and a file name on each row (`-r`, `-R`, `-H` or two paths); flags `-r -R -n -H -i -w -x -E -F -G -l -c` and `-e`; rows are `path:line:text` with `-n`, `path:text` without, a path per line with `-l` and `path:count` with `-c` (`-l` with `-c` is refused); context, files-without-match, only-matching, quiet, inverted, NUL output and `-h` are refused | compresses |
| `rg` | `rg PATTERN PATH...` (or `-e PATTERN PATH...`), with or without `-n`; flags `-n -i -S -s -w -x -F -H -u -L -l -c`, `-e -g -t -T -m` and the long forms of the same; `-l` lists paths and `-c` counts per file (not together); a path is required (rg reads stdin without one); a run whose rows carry no path (one file operand) stays raw; `-r`, `--pre`, `-z`, context, `-o`, `-v`, `--json`, `--heading`, `--files-without-match` and every other flag are refused | compresses |
| `rg-files` | `rg --files [PATH...]`; flags `--hidden --no-ignore --follow -u -L -g -t -T`; any other flag is refused | compresses |
| `git-grep` | `git grep -n PATTERN [-- PATHSPEC...]`; flags `-n -i -w -F -E -I`, `-e`; `-O`, `-h`, `-c`, `-l`, `-o`, `-v`, `-p`, context and every other option are refused | compresses |
| `ast-grep` | `ast-grep` | passes through |
| `file-read` | `cat FILE`, `head`/`tail` `[-n N | -nN | -N] FILE`, `sed -n 'A,Bp' FILE`, `jq . FILE`: one named file, no other option; `tail -f`/`-F`/`-c`/`+N`, `head -c`/negative `N`, `sed -i`/`-e`/`-f`/`-s` and any script but a print range, any jq filter but `.` and any jq option, several files, `-` and `/dev`, `/proc`, `/sys` are refused. Compacts a long (300+ lines) source or Markdown file, a `.log` file of 40+ lines (repeated lines folded) and a JSON document of 4 KiB or more (`.json` or `jq .`; schema view), and never a file whose name looks like a secret or env file (`.env`, `*.pem`, `*key*`, `*secret*`, `*credential*`, `.ssh/`, `.aws/`) or an operand that is not a regular file | compresses |
| `curl` | `curl [-sSiIfL] [--silent --show-error --include --head --fail --location --compressed] [-m N] [-X GET\|HEAD] URL`: one `http://` or `https://` URL with no credentials in it; every other option (`-o`, `-O`, `-d`, `--data*`, `-T`, `-F`, `-K`, `-H`, `-u`, `-b`, `-x`, `-k`, `-v`, a method other than GET or HEAD) and any other scheme are refused. Compacts a response of 2 KiB or more to the status line, key headers (credential-looking headers show `[redacted]`) and the ends of the body; a status of 400 or more stays raw. Never offered to the raw-output store | compresses |
| `wget` | `wget [-q] -O - URL` (also `-O-`, `-qO-`, `--output-document=-`): the document must go to standard output, since wget otherwise writes a file; any other option is refused. Same cap and redaction as `curl` | compresses |
| `glab` | `glab mr list\|view\|diff`, `glab issue list\|view`, `glab ci status\|list`, `glab pipeline list`; `-w`/`--web`, `--show-token`, `-l`/`--live` (also inside a short cluster such as `-cw`) and every other subcommand (`api`, `auth`, `config`, `variable`, `ci view`, `ci trace`, `job`, every write verb) refused. Redacts GitLab tokens, `PRIVATE-TOKEN`/`Bearer` values and URL credentials; cuts nothing. Never offered to the raw-output store | compresses |
| `agent-desktop` | `agent-desktop snapshot ...` and `agent-desktop find ...` with the subcommand first; `--debug`, `--screenshot`, `--help`, `--version` and any global flag before the subcommand refused. A successful envelope is re-encoded one line per element with every field kept (refs as `eN` under a legend, the common action list written once); a repeat read of the same window, in the same hook session or (for a direct call) the same working directory, is sent as a delta against the previous view ([agent-desktop guide](../guides/agent-desktop.md)). An error envelope, `find --count`/`--first` and any other subcommand stay raw | compresses |
| `wc` | `wc -l FILE...`; any other flag, `-` and `/dev`, `/proc`, `/sys` are refused | passes through |
| `dotnet-build` | `dotnet build` | compresses |
| `dotnet-test` | `dotnet test`; `-d`/`--diag` and `--results-directory` refused (green run only) | compresses |
| `dotnet-run` | `dotnet run` | passes through |
| `deno-test` | `deno test`; `--watch` and remote URLs refused (generic chain only) | compresses |
| `deno-check` | `deno check`; `--watch` and remote URLs refused (generic chain only) | compresses |
| `deno-lint` | `deno lint`; `--fix`, `--watch` and remote URLs refused (generic chain only) | compresses |
| `zig-build` | `zig build`; a `run*` step, `--watch` and `--test-cmd*` refused (generic chain only) | compresses |
| `zig-test` | `zig test`; `--watch` and `--test-cmd*` refused (generic chain only) | compresses |
| `flake8` | `flake8`, `python -m flake8` (generic chain only) | compresses |
| `pylint` | `pylint`, `python -m pylint` (generic chain only) | compresses |
| `unittest` | `python -m unittest` (generic chain only) | compresses |
| `tox` | bare `tox`, `run`, `list`, `config` with only `-e`, `-p`, `--recreate`; `exec`, `devenv`, `-x`, `--override`, `--` and URLs refused (generic chain only) | compresses |
| `nox` | `nox` (generic chain only) | compresses |
| `coverage` | `coverage report`, `coverage run -m pytest` or `-m unittest` (generic chain only) | compresses |
| `django-test` | `python manage.py test`, `django-admin test` (generic chain only) | compresses |
| `django-check` | `python manage.py check`, `django-admin check` (generic chain only) | compresses |
| `django-migrations` | `showmigrations`; `makemigrations` only with `--dry-run` and `--noinput`; `migrate` only with `--plan` (plus `--noinput`, `--skip-checks`) (generic chain only) | compresses |
| `stylelint` | `stylelint`, also `npx`; `--fix` and remote URLs refused (generic chain only) | compresses |
| `htmlhint` | `htmlhint`, also `npx`; `--fix` and remote URLs refused (generic chain only) | compresses |
| `html-validate` | `html-validate`, also `npx`; `--fix` and remote URLs refused (generic chain only) | compresses |
| `tailwindcss` | `tailwindcss`, `npx tailwindcss`, `npx @tailwindcss/cli`; `init`, `--watch`, `-w` and remote URLs refused (generic chain only) | compresses |
| `sass` | `sass` with an argument, also `npx`; `--watch`, `-i`, `--interactive`, `--stdin` and remote URLs refused (generic chain only) | compresses |
| `postcss` | `postcss` with an argument, also `npx`; `--watch`, `-w`, `-r`, `--replace` and remote URLs refused (generic chain only) | compresses |
| `next-lint` | `next lint`, `npx next lint`; `--fix`, `--fix-type`, `--watch` and remote URLs refused (generic chain only) | compresses |
| `js-check-script` | `npm run` or `bun run`, and `pnpm`/`yarn` with or without `run`, for the scripts `lint`, `typecheck`, `type-check`, `check` only; `--fix`, `--write`, `--watch` and remote URLs refused (generic chain only) | compresses |
| `knip` | `knip`, `npx knip`; `--fix`, `--fix-type`, `--watch`, `--config` and remote URLs refused (generic chain only) | compresses |
| `depcheck` | `depcheck`, `npx depcheck`; `--parsers`, `--detectors`, `--specials` and remote URLs refused (generic chain only) | compresses |
| `madge` | `madge --circular <path>`, `npx madge ...`; `--image`, config options and remote URLs refused (generic chain only) | compresses |
| `rspec` | `rspec`; `--init`, `-o`/`--out` refused (green run only; pending examples keep it raw) | compresses |
| `rubocop` | `rubocop`; `-a`, `-A`, `-x`, `--auto-correct*`, `--autocorrect*`, `-o`/`--out`, `--auto-gen-config` and any `--format` other than `clang` refused | compresses |
| `mise` | `mise` | passes through |
| `asdf` | `asdf` | passes through |
| `direnv` | `direnv` | passes through |
| `psql` | `psql` | passes through |
| `gcloud` | `gcloud` | passes through |
| `journalctl` | `journalctl`; not `-f`/`--follow` | passes through |
| `systemctl` | `systemctl` | passes through |
| `kubectl-top` | `kubectl top` | passes through |
| `xcodebuild` | `xcodebuild` with `build`, `test`, `build-for-testing`, `test-without-building`, `-list` or `-showBuildSettings`; `clean`, `archive`, `install`, `-exportArchive` and remote URLs refused | passes through |
| `pod` | `pod install`, `update`, `outdated`, `list`, `spec lint`, `repo update`, `repo list`; `repo push`, `trunk` and other `spec` forms refused | passes through |
| `expo` | `expo` or `npx expo` with `prebuild`, `export`, `doctor`, `install`, `config`, `diagnostics`; `start`, `upgrade`, `run:*`, `--watch` and remote URLs refused | passes through |
| `react-native` | `react-native` or `npx react-native` with `run-android`, `run-ios`, `bundle`, `info`, `doctor`; `start`, `log-*`, `--watch` and remote URLs refused | passes through |
| `adb` | `adb devices`, `install`, `logcat -d` or `logcat -t N`; streaming `logcat`, `push`, `pull`, `bugreport` refused | passes through |
| `detox-test` | `detox test` or `npx detox test`; `--watch` and remote URLs refused (generic chain only) | compresses |
| `detox-build` | `detox build` or `npx detox build`; `--watch` and remote URLs refused (generic chain only) | compresses |
| `pip-install` | `pip install` (install form; a run with a WARNING or an error passes through) | compresses |
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

**Quoted words (2026-11-06).** One command (not a chain or shape) may hold
closed `'...'` words, `"..."` words with no `$`, backtick, `\` or `!`, and an
exact trailing `2>/dev/null` and/or `|| true`; it is rewritten to
`'<bin>' compact [--discard-stderr] [--exit-zero] -- '<w0>' '<w1>' ...`.
The read-only approval path of Codex and Cursor still refuses all of them.
Threat note: [compactor-default-on-adr.md](../design/compactor-default-on-adr.md).

| harness | shell rewrite | status | limit |
|---|---|---|---|
| claude-code | `PreToolUse` on `Bash` → `updatedInput.command`, every sibling field kept, no `permissionDecision` | Wired, on by default | The plugin's `hooks.json` carries the `Bash` PreToolUse entry running `downshift claude-code`. Verified live on 2026-10-20. If Claude Code ever ignores `updatedInput` without a `permissionDecision`, the command runs unchanged: nothing breaks, nothing is saved. Permissions: no allow rule is installed or required. The rewrite is skipped when a deny or ask rule in the managed, user (`CLAUDE_CONFIG_DIR` honoured), project or local settings matches the **original** command (as written and normalised); an unreadable or invalid settings file also means no rewrite. The hook emits no `permissionDecision`, so Claude Code's own permission flow, including any approval prompt, applies to the rewritten command as it would to the original. Deny rules from `--disallowedTools`, Agent SDK `disallowedTools`, `--settings <file>`, server-managed policy or MDM cannot be read, and they would not match the wrapped form; in `bypassPermissions` or `dontAsk` mode (or any mode the adapter does not know) nothing prompts, so the rewrite is limited to one read-only command, as for Codex (skip `not_read_only`). A call with `run_in_background` set is never wrapped (skip `background`): the compactor holds output until the command exits. Never add a broad `Bash(downshift compact:*)`. |
| codex | `PreToolUse` on `Bash` (also reported for `exec_command`) → `permissionDecision: "allow"` plus `updatedInput.command`, every sibling field kept | Wired, Unverified live | `downshift setup` writes a `^Bash$` entry running `downshift codex` next to the `Agent` one. Codex honors `updatedInput` only together with `permissionDecision: "allow"`, so a rewritten call is approved by the hook and Codex does not ask the user (`RewriteImpliesApproval`). The shared runner therefore rewrites only one read-only command (`execwrap.ReadOnly`): no shell metacharacter at all (no `;`, `&&`, `\|\|`, `\|`, `&`, `$(`, backtick, redirection, quote or newline), no leading `NAME=value`, and one of `git status/log/diff/show` (without `--output`, `--ext-diff`, `--textconv`), `git branch` and `git remote` with listing flags only, `git stash list`, `git rev-parse`, `git ls-files`, `gh pr view/list/status/diff/checks`, `gh issue view/list/status`, `gh run/workflow/repo/release/gist view/list`, `gh search`, `go version`, `go doc`, `go env` (without `-w`/`-u`), `go list` (without `-toolexec`, `-exec`, `-export`, `-compiled`, `-mod`), `mise/rtx/asdf list/ls/current`, `direnv status` and `ls`. Every runner, build, test and install family (`go test`, `make`, `npm test`, `uv run`, ...), every chain or compound shape and every mutating subcommand is left untouched with no `permissionDecision`, so Codex asks as usual; the skip is logged as `not_read_only`. An unchanged call emits `{}`. Codex has no deny or ask rules the adapter can read, so `ShellBlocked` is always false. Unverified live until a Codex session runs a wrapped `git status` (the hook must be trusted in `/hooks`). |
| cursor | `preToolUse` on `Shell` → `updated_input.command`, every sibling field kept, no `permission` field | Wired | Unverified live; Cursor ignored `updated_input` for Task on some plans, Shell needs its own proof. `downshift setup` writes the `Shell` entry in `hooks.json`. No deny/ask rule is read (Cursor has no readable rule file), so a rule on the original text could miss the wrapped form (`UnknownUserRules`): the preToolUse rewrite is limited to the same single read-only command set as Codex (`execwrap.ReadOnly`, see the codex row), and every other command, such as `go test` or `gh repo delete x --yes`, is left as written (skip `not_read_only`). Advise fallback: `beforeShellExecution` (matcher `*`) answers `agent_message` with no `permission` field (never `allow`, so Cursor keeps its own approval flow), naming the `downshift compact` form for an allowlisted family (`CanAdviseShellWrap`); the advise is not limited to the read-only set. It never denies and cannot rewrite. Any other shell command gets the neutral `{}`. Unverified live: that Cursor shows `agent_message` when the answer carries no permission, and that `{}` defers to its own flow. |
| opencode | `shell.create.before` → the command is replaced in place (the session tag comes from `tool.execute.before`, which runs first) | Wired by the bridge; live-checked on `git status` (rewritten through the plugin, output unrecognised, passed through raw). `go test` compaction through the plugin not yet checked live | The rewrite runs only for allowlisted families with no shell metacharacters; any permission rule that is not `allow` blocks it, and so does a source the check cannot read or parse. The check reads `OPENCODE_CONFIG_CONTENT`, `OPENCODE_PERMISSION`, `OPENCODE_CONFIG`, the config dirs (`$XDG_CONFIG_HOME/opencode`, `~/.config/opencode`, the managed dir, `OPENCODE_CONFIG_DIR`), and for the session directory and the shell's directory each `opencode.jsonc`, `opencode.json` and `.opencode` folder from that directory up to the filesystem root. Markdown files under `agent`, `agents`, `mode` and `modes` that declare `permission` in their frontmatter count (the scan reads JSONC). Any doubt keeps the command unchanged: no known directory or a relative one, a linked `agent`, `agents`, `mode` or `modes` folder or an entry in one, a frontmatter the scan cannot read, a config file over 1 MiB, a non-regular file, or a macOS managed profile (`/Library/Managed Preferences/ai.opencode.managed.plist`, also under any user folder there). Each config read is capped at 1 MiB. The bridge applies a rewrite only when the original command has no quote, `$`, backtick, backslash or control character, only runs of ASCII spaces differ, and each wrapper is in command position (at the start, or after `;`, `&&`, `||` or `|`, optionally after `NAME=value` words) and names the pinned binary (`DOWNSHIFT_BIN`, or `downshift` resolved from PATH once at startup, skipping relative entries and files without an execute bit). A pending session tag of a shell call expires after 30 s, at most 500 are held in total (oldest dropped), and the bridge waits at most 2 s for the hook. |
| antigravity | `PreToolUse` on `run_command` → `decision: "allow"` plus `overwrite.CommandLine`, which the vendor shallow-merges into `toolCall.args`, so sibling arguments stay as sent | Wired, documented, not live-verified | Contract from the vendor hooks page (antigravity.google/docs/hooks); no Antigravity build with a hook that could be exercised non-interactively was available on 2026-11-06, so no run has shown `overwrite.CommandLine` honored. `downshift setup` writes a `run_command` entry next to `invoke_subagent`. The answer approves the call (`decision: "allow"`) and the user's rules cannot be read (`UnknownUserRules`), so the shared runner rewrites only one read-only command (`execwrap.ReadOnly`, see the codex row); every other command is left as written (skip `not_read_only`). `ShellBlocked` is always false. |
| kirocrew | not wired | No rewrite channel | Policy-mode only (allow or block), so it can never rewrite a command. |
| grok | none | No hook | Config only. |

`downshift compact -- <argv>` works the same from any shell, harness or not.

**Known layering gap (OpenCode shell guard).** The list of OpenCode config sources the guard reads (the env names, the config file and folder layout, the managed paths) is code in `internal/adapters/opencode`, not data in `HarnessCapabilities`, and the adapter decides from it whether the compactor stays off. It predates v0.8.1 and grew with it. Moving those facts into capability data is open work.

**Re-check on a live session (C14).** A unit test cannot show that Claude Code applies `updatedInput.command` when the hook returns no `permissionDecision`. To check it after a Claude Code upgrade:

1. Make sure the Downshift plugin is installed and no deny or ask rule matches `go test`. No allow rule is needed, and none should be added for `downshift compact`.
2. Start a new Claude Code session in a Go repository and ask it to run `go test ./...` (a repository with a dozen or more packages, all passing).
3. Look at the tool call Claude Code shows. The command must read `'<path>/downshift' compact -- go test ./...`, and a passing result must end with a `[downshift compact: compacted N to M bytes]` line.
4. Run `downshift context metrics`: `exec_metrics.commands` must have grown. With `DOWNSHIFT_EXEC=0` in the environment the command must run as typed.

If the command runs as typed with the hook on, Claude Code ignores the rewrite without a decision: nothing breaks and nothing is saved, and this feature should stay documented as not working there.

## Several hooks rewriting one call (2026-11-06)

When another hook on the same event and tool also returns a rewritten input, the
harness picks one. Downshift computes the overlap for every harness whose hook
file `doctor` reads: each Downshift hook that rewrites input carries
`setup.Hook.Rewrites` (`model` or `shell`) in `setup.Targets`, and `doctor` warns
(`hook overlap`) when a recognised rewriter of the same kind shares its event and
tool. What the harness then does is a harness fact, and only one is measured.

| Harness | Downshift hooks tagged as rewriters | Which rewrite runs when two hooks answer | Evidence |
|---|---|---|---|
| claude-code | `Agent\|Task` (model), `Bash` (shell) | Not merged, not guaranteed by the documentation; measured on 2.1.319: the hook that returns last, not the first in the file. A permission `allow` from either hook still applies | [hook coexistence](../evidence/hook-coexistence-2026-11-07.md) |
| cursor | `Task` (model), `Shell` (shell) | not measured | none |
| codex | `(^Agent$\|.*spawn_agent$)` (model), `^Bash$` (shell) | not measured | none |
| antigravity | `invoke_subagent` (model), `run_command` (shell) | not measured | none |
| kirocrew | none: the hook cannot rewrite in place | not applicable | none |
| opencode | none in a hook file: the bridge is a plugin | not checked by `doctor` | none |

On Claude Code this qualifies the sentence "No Bash allow rule is needed" in the
shell-rewrite section below: Downshift emits no `permissionDecision`, so a hook
that answers `allow` for the same call also approves Downshift's rewritten
command, including when that hook's own rewrite was the one discarded.

## MCP registration (`setup`, on by default; `--no-mcp` skips it, 2026-10-28)

Only Claude Code (through `claude mcp add -s user`, detection by reading `~/.claude.json`) and Codex (marked block in `~/.codex/config.toml`) are registered; the file formats of Cursor, OpenCode, Antigravity and Kiro are in `docs/internal/plans/mcp-registration.md` and arrive in a later stage. Unverified: what `claude mcp add` does on a duplicate name (setup never adds over an existing entry), where `.claude.json` lives under `CLAUDE_CONFIG_DIR`, and whether Codex loads the server without a trust step.

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
