# Harness x plan x rewrite-honored matrix

Downshift by Tiago de Carvalho Vilas Boas
https://github.com/tiagovilasboas/downshift

## Status per harness

Each harness carries exactly one of three status words (owner decision 2026-10-17); the table below it keeps the per-plan facts.

- **Completed:** rewrite observed honored on a real child, usage and child end recorded, plugin and session policy.
- **Functional:** the hook emits the rewrite on a real spawn and passes the shared conformance suite; observation is partial (honored is inferred or unconfirmed, no child usage).
- **Experimental (proof of concept):** policy mode or one observed child on one build; shipped, but no production claim.

`cmd/downshift/conformance_test.go` drives the same 8 routing scenarios against every registered hook, so each harness below, whatever its status, computes every feature (layering rule); the status says how much has been verified on the real harness, not how much code it runs.

| harness | status | why | evidence |
|---|---|---|---|
| claude-code | Completed | Rewrite observed honored (2026-10-06, API-reported child model); usage priced from the child transcript at `SubagentStop`; child end recorded; plugin and session policy ship. Failure payload is still GAP-CHILD-FAILURE (see "Child end" below). | [rewrite honored](../evidence/claude-code-rewrite-honored-2026-10-06.md), [billing audit](../evidence/billing-gap-audit.md), [routing A/B](../evidence/subagent-routing-ab-2026-10-15.md) |
| codex | Functional | Rewrite emitted on a real spawn; honored is inferred from the next spawn, not observed; child end parser unverified, no usage source. | [codex rewrite](../evidence/codex-rewrite-honored-2026-10-02.md) |
| cursor | Functional | Rewrite emitted on a real spawn; the child executor model is absent from the transcript, so honored is unconfirmed (discarded on Free and legacy Pro); end parser unverified, no usage; native quota collector blocked on Cursor 3.24.9. | [cursor rewrite](../evidence/cursor-rewrite-honored-2026-10-10.md), [gap diagnostic](../evidence/cursor-gap-diagnostic.md) |
| kirocrew | Experimental | Policy mode: block and retry observed once on Kiro CLI 2.29.0, no in-place rewrite; no child end recorded. | [native hook](../evidence/kirocrew-native-hook-2026-10-10.md) |
| antigravity | Experimental | One observed child on desktop 2.21.1 (requested `pro`, executed `flash_lite`); the `overwrite` envelope is not a documented vendor field; no child end recorded; quota mapping unresolved. | [executor ack](../evidence/antigravity-executor-ack.md), [runtime source](../evidence/antigravity-runtime-source.md) |
| grok | Config only | No hook rewrite; reasoning is pinned in `config.toml`. | [install.md](../guides/install.md#grok-cli-config-not-hook) |

"Rewrite honored" means the harness executor applied the hook's model
rewrite to the child subagent, not just that downshift emitted one.
`outcome: "rewrite_emitted"` proves emission only (docs/reference/session-models.md:17).

| harness | plan / build | rewrite honored? | evidence / limit |
|---|---|---|---|
| claude-code | paid: Pro / Max / Teams / API | Yes, 2026-10-06 (two spawns; API-reported model in the child transcript) | `updatedInput.model` must be a family name (`haiku`/`sonnet`/`opus`/`fable`); a full id is rejected and blocks the spawn. The catalog `native_name` supplies it. Write-up: `docs/evidence/claude-code-rewrite-honored-2026-10-06.md`. Without `session-models.json` the session is inferred from the current model (downshift only, never above it); a list is needed only to allow upshifts. |
| claude-code | Free | No: no real subagents | Sandboxed, no egress; subagents need paid (README.md:728) |
| cursor | Free | No: silently discarded | `updated_input.model` ignored, known issue (README.md:734) |
| cursor | Pro legacy (request-based) | No: silently dropped | Task model takes only `"fast"` (README.md:735) |
| cursor | Pro / Ultra usage-based | Unconfirmed | Actual child-model proof remains P1.6. Separately, Cursor 3.24.9 blocks the external native quota collector before transport; see [diagnostic](../evidence/cursor-gap-diagnostic.md). |
| codex | multi_agent_v2 + hooks on | Yes, inferred (2026-10-02) | Inferred, not observed: same-session follow-up: spawn requested `gpt-5.6-terra` after hook emitted Terra (`docs/evidence/codex-rewrite-honored-2026-10-02.md`). Payload >1MB still fail-open. Schema still evolving upstream. |
| kirocrew | Kiro CLI 2.29 native spec hook; MCP `spawn_run` with explicit `task` + `model` | Policy/block + retry observed 2026-10-10; no in-place rewrite | Persistent `agent.kiro_hooks.preToolUse` + executable wrapper; no `updated_input` channel. Native `use_subagent` without child model is outside coverage. `hooks.json` tool-call hooks are informational and do not enforce blocks. [Native evidence](../evidence/kirocrew-native-hook-2026-10-10.md); [setup and limits](../guides/install.md#kirocrew-policy-mode-through-the-native-kiro-cli-hook). |
| antigravity | desktop 2.21.1, `invoke_subagent` | Yes, observed 2026-10-09 | One exact child: original `pro`, executed `flash_lite`, native executor/generation model `gemini-3.5-flash-lite`. [Evidence and QA](../evidence/antigravity-executor-ack.md). Manual read-only, build-specific sensor; quota/picker/alias mapping remains unresolved. |
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

## Decision linkage (current model and post-spawn cost)

Observed on claude-code 2026-10-12: the PreToolUse payload carries
`tool_use_id` and `transcript_path` but no session model.

- **Current model.** A Task call without `model` (agent type empty,
  `general-purpose`, `claude`, and the built-in `Explore` and `Plan`) runs on
  the session model. The adapter reads it from the last assistant message in
  the tail of `transcript_path` (1 MiB, model id only). Other agent types may
  pin their own model (`statusline-setup` runs on Sonnet, `claude-code-guide`
  on Haiku, a custom type on its frontmatter) and stay `unknown`. `Explore`
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
  held and allow decisions (no `rewrite_honored` claim for those). Lines
  written before this field fall back to the session heuristic.
- **Other harnesses.** cursor and codex expose `tool_use_id` on decisions
  (documented in their hook payloads). Their end hooks (see "Child end" below)
  join on it when the end payload repeats the id.
- **codex current model.** A `spawn_agent` call without `model` runs on the
  parent's current model: 22 of 24 such spawns in the operator's rollouts
  (2026-10-12; the other mismatches were Downshift's own rewrites). The
  adapter reads the last `turn_context.payload.model` from the tail of
  `transcript_path`. `gpt-6.1-sol` is registered for identity and tier; list
  prices are absent until a vendor snapshot matches exactly, so cost telemetry
  stays unknown for that id. `gpt-reserve` is not in the catalog yet, so they
  still log as `unknown` with `requested_model_raw` set.
- **antigravity current model.** `Subagents[].Model` arrives in the payload
  (2026-10-09 run asked for `pro`) but was never exposed to telemetry; it is
  now, for the subagent whose decision is recorded (the last with a prompt).
  The documented top-level `modelName` is not read until a real payload
  confirms it.
- **cursor current model.** In 144 captured Task payloads (2026-10-09/10) the
  top-level `model` was always empty; the subagent `model` was a real id,
  `inherit` (28) or absent (30). With `model_id` on the payload, `inherit` is
  resolved like a missing `tool_input.model` and logged as that parent id; without
  `model_id` it logs as `unknown`.
- **Unrecognised models (all harnesses).** When the requested model is
  non-empty but not in the catalog, the shared runner records it in
  `requested_model_raw` if it looks like a model id (64 chars,
  `[A-Za-z0-9._:/-]`), else `invalid`.

## What each harness exposes for session discovery (2026-10-11/12)

Findings from the official docs plus local read-only inspection. Rows say what a
hook or local source can tell Downshift; anything not listed was not found.
`InferSession` (HarnessCapabilities) is on only where the current model proves
access to the tiers below it.

| harness | current model in hook | available models | quota | session-start hook | InferSession |
|---|---|---|---|---|---|
| claude-code | `SessionStart.model` (optional); the Agent `tool_input.model` when the parent set it; `PostToolUse.tool_response.resolvedModel` | No listing for subscriptions; `availableModels`/`deniedModels` exist only in managed settings; the Models API needs `ANTHROPIC_API_KEY` | statusline `rate_limits.five_hour` / `seven_day` (global, Pro/Max only, after the first response); not per model | `SessionStart` (add context, `CLAUDE_ENV_FILE`) | yes |
| codex | `model` on every payload; effort only if the parent put `reasoning_effort` in `spawn_agent` | `models_cache.json` (offline; `hide` means not shown in the picker, not unusable); app-server `model/list` is experimental | per meter (`codex`, `premium`), never per model | `SessionStart` can only add context | no |
| cursor | `model`, `model_id`, `model_params`; `subagent_model` on `subagentStart` | `cursor-agent models` (text, needs login; failed here with "Authentication required"); Cloud `/v1/models` needs an API key | Cursor Models vs Other Models pools, UI/dashboard only; CLI `/usage` is interactive | `sessionStart` (env reaches later hooks; not for cloud agents) | no |
| antigravity | `modelName` (parent) and `conversationId`; the adapter does not read them yet | `agy --output-format json models` (parent picker); no alias-to-model or pool mapping | `agy -p /usage --output-format json` per account, group and window; not tied to an alias | none (Pre/PostToolUse, Pre/PostInvocation, Stop) | no |
| kirocrew | none in the payload; `session_id` maps to `~/.kiro/sessions/cli/<id>.json` (`agent_name`, `model_id`, undocumented) | `kiro-cli chat --list-models --format json` (10 models plus `auto`, with `rate_multiplier`) | no supported non-interactive credit source | `agentSpawn` (renamed `SessionStart` in V3); stdout goes to the agent context; any non-zero exit blocks | no |

Consequences: Claude Code is the only harness where the inferred ladder is
safe; every other harness needs a real list (hook payload, discovery cache or
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
| cursor | `subagentStop` | none (no usage field is read) | **unverified: needs a real payload.** Linked only when the payload names the spawn tool call (`tool_use_id` / `tool_call_id`); Cursor sends no session id, so otherwise nothing is recorded. No model is read. |
| antigravity | none | none | no documented subagent-end payload (PostToolUse exists; its result fields are not documented). Nothing recorded. |
| kirocrew | none | none | PostToolUse `tool_response` not confirmed (see `evidence/vendor-hook-docs-2026-10-11.md`). Nothing recorded. |

Where `ChildEndEvent` is empty, `downshift end <harness>` exits 0, records nothing
and says so on stderr.

## Child-end line (U2, 2026-10-13)

`HarnessCapabilities.CanShowChildLine` says whether the hook protocol can show
one line when a subagent ends. Claude Code: true (`systemMessage` on
PostToolUse and SubagentStop), pending a live check. cursor, codex, antigravity,
kirocrew: false; the line is still computed by `telemetry.ChildEndLine` (by the
shared recorder, for every harness) and no adapter prints it. `DOWNSHIFT_QUIET=1` turns it off. See
[session-models.md](session-models.md#what-downshift-shows-while-it-works).

## Session view (U3, 2026-10-13)

`downshift session` (tree, `--why`, `--pause`) is shared code; what differs is
whether a harness gives it a session id. `HarnessCapabilities.SessionEnv` names
the variable a harness exports to commands run from it: Claude Code
`CLAUDE_CODE_SESSION_ID` (seen set in a Bash tool call on Claude Code 2.1.289);
others declare none and the command uses the latest session in the log. The
hook payload carries a session id for Claude Code and Codex only, so only they
group in the tree and can be paused per session. Cursor, Antigravity and
KiroCrew events carry none: computed and logged, not groupable, not pausable
(the global `DOWNSHIFT_NO_ROUTE=1` still applies). The `/downshift` slash
command is Claude Code only. See [session-view.md](../guides/session-view.md).

## Public score coverage (capability path)

Mapped means an entry in `internal/scoreindex/idmap.json` names the session id.
Whether a score row exists at runtime also depends on the public source listing
that model at the last refresh; that is not checked here. Live state:
`downshift models scores --status`. Method: [scores.md](scores.md#the-id-map).

| harness | session ids with an id-map entry | not mapped (reported `uncovered`) |
|---|---|---|
| claude-code | `claude-haiku-4-5`, `claude-haiku-5-5`, `claude-sonnet-5-5`, `claude-opus-5-5`, `claude-fable-5-1`, `grok-4.6` | any other id |
| cursor | `claude-sonnet-5-5-high`, `claude-opus-5-5-medium`, `claude-fable-5-1-thinking-high`, `claude-opus-5-thinking-high`, `claude-4.5-haiku-thinking`, `claude-4.5-sonnet-thinking`, `claude-4.5-opus-high-thinking`, `cursor-grok-4.6-medium`, `grok-4.7-xhigh`, `gpt-5.6-sol-high`, `gemini-3.8-flash-high`, `glm-5.2-high`, `muse-spark-1.3-max` | any other id, including `composer-2.5` |
| kirocrew | `claude-sonnet-5-5`, `claude-opus-5-5`, `claude-haiku-4-5` | any other id |
| codex | `gpt-6-luna`, `gpt-6-sol`, `gpt-6-astra`, `gpt-5.6-luna`, `gpt-5.6-terra`, `gpt-5.6-sol` | any other id |
| antigravity | `flash` | `flash_lite`, `pro` (no entry) |

`composer-2.5`, `flash_lite` and `pro` have no entry anywhere in `idmap.json`, so
they are reported uncovered rather than guessed. An uncovered candidate makes the
capability path abstain for that decision (see [scores.md](scores.md#when-it-abstains)).

## Bundled mod: cache-tax (2026-10-17)

The Claude Code plugin's `hooks/hooks.json` lists one mod module,
[`hooks/cache-tax/`](../../hooks/cache-tax/README.md) (third-party, MIT,
vendored and pinned; owner decision 2026-10-17). It is not a routing feature
and nothing in the shared layers computes it: a mod is code Claude Code runs
in its own process, so it exists only where mods load.

| harness | bundled mod | why |
|---|---|---|
| claude-code | loads with the plugin on 2.1.287+ (CLI, Desktop Code tab, VS Code extension; hooks also run in `claude -p`, nothing is drawn there) | mods are a Claude Code plugin feature; `setup` cannot write one |
| codex, cursor, kirocrew, antigravity, grok | not available | no mod system; the cold-cache question is the harness's own |

What it does is in its README; the one fact that matters here is that the
warming ping spends usage and is armed by the user (or for three hours after a
detected cold write), while the guard itself sends nothing.

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
- Vendor hook docs vs adapters (search summaries, 2026-10-11): [vendor-hook-docs](../evidence/vendor-hook-docs-2026-10-11.md)
- Antigravity observed child, dated build and evidence limits: [native evidence](../evidence/antigravity-executor-ack.md) and [final QA](../evidence/final-gap-qa.md)
- Grok config-only: README.md:520-531, README.md:691, README.md:1236
- Emission vs honored: docs/reference/session-models.md:17; allowlist: docs/reference/session-models.md:34
