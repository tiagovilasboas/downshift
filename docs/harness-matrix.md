# Harness x plan x rewrite-honored matrix

harness-downshift by Tiago de Carvalho Vilas Boas
https://github.com/tiagovilasboas/downshift

"Rewrite honored" means the harness executor applied the hook's model
rewrite to the child subagent, not just that downshift emitted one.
`outcome: "rewrite_emitted"` proves emission only (docs/session-models.md:17).

| harness | plan / build | rewrite honored? | evidence / limit |
|---|---|---|---|
| claude-code | paid: Pro / Max / Teams / API | Yes, 2026-10-06 (two spawns; API-reported model in the child transcript) | `updatedInput.model` must be a family name (`haiku`/`sonnet`/`opus`/`fable`); a full id is rejected and blocks the spawn. The catalog `native_name` supplies it. Write-up: `docs/evidence/claude-code-rewrite-honored-2026-10-06.md`. Needs `session-models.json` or the hook never rewrites. |
| claude-code | Free | No: no real subagents | Sandboxed, no egress; subagents need paid (README.md:728) |
| cursor | Free | No: silently discarded | `updated_input.model` ignored, known issue (README.md:734) |
| cursor | Pro legacy (request-based) | No: silently dropped | Task model takes only `"fast"` (README.md:735) |
| cursor | Pro / Ultra usage-based | Unconfirmed | Actual child-model proof remains P1.6. Separately, Cursor 3.24.9 blocks the external native quota collector before transport; see [diagnostic](evidence/cursor-gap-diagnostic.md). |
| codex | multi_agent_v2 + hooks on | Yes, inferred (2026-10-02) | Inferred, not observed: same-session follow-up: spawn requested `gpt-5.6-terra` after hook emitted Terra (`docs/evidence/codex-rewrite-honored-2026-10-02.md`). Payload >1MB still fail-open. Schema still evolving upstream. |
| kirocrew | Kiro CLI 2.29 native spec hook; MCP `spawn_run` with explicit `task` + `model` | Policy/block + retry observed 2026-10-10; no in-place rewrite | Persistent `agent.kiro_hooks.preToolUse` + executable wrapper; no `updated_input` channel. Native `use_subagent` without child model is outside coverage. `hooks.json` tool-call hooks are informational and do not enforce blocks. [Native evidence](evidence/kirocrew-native-hook-2026-10-10.md); [setup and limits](install.md#kirocrew-policy-mode-through-the-native-kiro-cli-hook). |
| antigravity | desktop 2.21.1, `invoke_subagent` | Yes, observed 2026-10-09 | One exact child: original `pro`, executed `flash_lite`, native executor/generation model `gemini-3.5-flash-lite`. [Evidence and QA](evidence/antigravity-executor-ack.md). Manual read-only, build-specific sensor; quota/picker/alias mapping remains unresolved. |
| grok | any (single model grok-4.6) | No hook rewrite: config only | PreToolUse is allow/deny; pin `reasoning_effort` in config.toml (README.md:520-531) |

"Inferred" means a later spawn in the same session asked for the model the
hook had written (`rewrite_honored_inferred` in `stats --export`). It is
correlation, not a read of the child's actual model. Only applied rewrites
(outcome `rewrite_emitted`, no guardrail correction) count; allow, blocked
and held events never do.

## Decision linkage (current model and post-spawn cost)

Observed on claude-code 2026-10-12: the PreToolUse payload carries
`tool_use_id` and `transcript_path` but no session model.

- **Current model.** A Task call without `model` (agent type empty,
  `general-purpose` or `claude`) runs on the session model. The adapter reads
  it from the last assistant message in the tail of `transcript_path` (1 MiB,
  model id only). Other agent types may pin their own model and stay
  `unknown`.
- **Cost linkage.** Every decision records `tool_use_hash`. PostToolUse
  repeats the id, so `resolved`, `child_ended` and, through the agent id,
  `usage` attach to the exact decision even with parallel spawns, including
  held and allow decisions (no `rewrite_honored` claim for those). Lines
  written before this field fall back to the session heuristic.
- **Other harnesses.** cursor and codex expose `tool_use_id` on decisions
  (documented in their hook payloads). Neither has a post-spawn cost hook yet,
  so nothing joins on it there today.
- **codex current model.** A `spawn_agent` call without `model` runs on the
  parent's current model: 22 of 24 such spawns in the operator's rollouts
  (2026-10-12; the other mismatches were Downshift's own rewrites). The
  adapter reads the last `turn_context.payload.model` from the tail of
  `transcript_path`. `gpt-6.1-sol` and `gpt-reserve` are not in the catalog
  yet, so they still log as `unknown` with `requested_model_raw` set.
- **antigravity current model.** `Subagents[].Model` arrives in the payload
  (2026-10-09 run asked for `pro`) but was never exposed to telemetry; it is
  now, for the subagent whose decision is recorded (the last with a prompt).
  The documented top-level `modelName` is not read until a real payload
  confirms it.
- **cursor current model.** In 144 captured Task payloads (2026-10-09/10) the
  top-level `model` was always empty; the subagent `model` was a real id,
  `inherit` (28) or absent (30). Cursor sends no transcript path, so
  `inherit` cannot be resolved to the session model; it is logged as
  `requested_model_raw: "inherit"`.
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

## Revalidation

Every TBD cell must be re-checked per harness release: hook contracts
evolve upstream and a green cell can regress silently. Failing proof,
the tool fails open and the spawn runs unchanged (README.md:755).

A 2026-10-09 sample of Codex `spawn_agent` inputs recorded key sets only
(`agent_type`, `message`, `task_name`, and sometimes `fork_turns`). It did
not include hook stdin, `transcript_path`, a model field, or executor
acknowledgement, so the Codex row above is not refreshed. See
[closeout-recheck-2026-10-09.md](evidence/closeout-recheck-2026-10-09.md).

## Sources (README/docs only, no vendor claims)

- Claude paid: emission only until P1.3 (docs/evidence/rewrite-honored-protocol.md); plan limits: README § Plan compatibility
- Cursor free/legacy discard: README.md:734-735, README.md:738, README.md:1012-1013
- Codex flags + honored: README.md:744, README.md:746; evidence 2026-10-02: docs/evidence/codex-rewrite-honored-2026-10-02.md; evolving schema: README.md:373
- KiroCrew adapter capability and integration limits: [native hook setup](install.md#kirocrew-policy-mode-through-the-native-kiro-cli-hook), [routing guia](kirocrew-routing-guide.md), `internal/adapters/kirocrew/kirocrew.go`. A hook invocation counter or direct adapter smoke test does not prove live enforcement or the served child model.
- Vendor hook docs vs adapters (search summaries, 2026-10-11): [vendor-hook-docs](evidence/vendor-hook-docs-2026-10-11.md)
- Antigravity observed child, dated build and evidence limits: [native evidence](evidence/antigravity-executor-ack.md) and [final QA](evidence/final-gap-qa.md)
- Grok config-only: README.md:520-531, README.md:691, README.md:1236
- Emission vs honored: docs/session-models.md:17; allowlist: docs/session-models.md:34
