# Claude Code: how Downshift routes subagents here

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

Claude Code is the primary harness: the only one with an inferred session list, a usage hook, a
child-end line and `/downshift`. This page says what the adapter can and cannot know, with the code
that decides each point. Every claim below is a code path or a dated observation; where the real
model is a guess, the page says so. Install steps: [install.md](install.md). Shared rules:
[session-models.md](../reference/session-models.md), [harness-matrix.md](../reference/harness-matrix.md).

## 1. opusplan is Claude Code's policy; Downshift routes under it

Code: `cmd/downshift/setup_targets.go` (`Defaults` of the `claude-code` target).

`opusplan` is a native Claude Code value for the `model` setting: Opus while plan mode is on, Sonnet
once you approve the plan and execute. It governs the **main session**. Downshift never replaces it
and never touches the main session's model at runtime: its PreToolUse hook sees only `Agent`/`Task`
spawns and applies the orchestrator's model choice to **subagents**, within your declared policy. The two layers compose: a subagent spawned
during plan mode would inherit Opus, the adapter reads that resolved model from the transcript and
routes the spawn as usual. That composition follows from the code path; a live plan-mode spawn has
not been observed yet (session-models.md, "Main session policy").

## 2. How the reference (baseline) model is found

Code: `internal/adapters/claudecode/claudecode.go` (`resolveModel`), `sessionmodel.go`,
`configuredmodel.go`; shared reader `internal/hookutil/transcript.go` (`TranscriptModel`).

The PreToolUse payload carries `transcript_path`, `cwd`, `permission_mode` and `tool_use_id`, but no
session model (observed 2026-10-12). For a call without `tool_input.model`, the adapter resolves the
model the child would otherwise run on, in Claude Code's own order:

1. `tool_input.model`, then the payload `model`: a requested model, not a guess.
2. Only for an **inheriting agent type** (`inheritingAgentTypes`: no `subagent_type`,
   `general-purpose`, `claude`, `Explore`, `Plan`) or an agent whose definition inherits
   (`agentdef.go`: `<cwd>/.claude/agents` and its parents, the user `agents` directory, or an
   installed plugin's `agents` for a `plugin:agent` type; no `model:` or `model: inherit`). An agent
   that names a model, and the built-ins `claude-code-guide` (Haiku) and `statusline-setup`
   (Sonnet), have that model as a known baseline and are left alone (`AGENT_PINS_MODEL`). A type
   with no definition found keeps an unknown baseline.
3. `CLAUDE_CODE_SUBAGENT_MODEL` (`core.ClaudeCodeCaps.ModelOverrideEnv`) when set and not `inherit`;
   `Explore` and `Plan` take it only under `CLAUDE_CODE_SUBAGENT_MODEL_FORCE`.
4. The transcript tail (1 MiB, `TranscriptTailBytes`): `message.model` of the assistant line that
   carries this call's `tool_use_id` (the parent's model at spawn time; waited for up to 400 ms when
   it is not on disk yet), else of the last `assistant` line, skipping `<synthetic>`. Only the id is read; content is never stored. If the file exists but has
   no assistant model yet, the read is retried for up to 400 ms (`transcriptWaitTotal`), once per path.
5. Settings fallback (`configuredModel`), needs the catalog: `ANTHROPIC_MODEL`, then `model` in
   `<cwd>/.claude/settings.local.json`, `<cwd>/.claude/settings.json`, then the user `settings.json`
   (`CLAUDE_CONFIG_DIR` honoured); first file that sets it wins. `opusplan` resolves to Opus when
   `permission_mode` is `plan`, Sonnet otherwise; `sonnet`/`opus`/`haiku`/`fable` map to the
   catalog's model of that family (`familyAliases`); a full id the catalog knows passes through;
   `default` or anything unrecognised is unknown. A hit here sets `Decision.Baseline.FromSettings`
   and `Decision.Baseline.Guessed`, logged as reason code `BASELINE_FROM_SETTINGS`.
6. `Explore` under a Fable main model is forced unknown (`exploreMayLeaveTheMainModel`): Claude Code
   may run Explore on Opus there (subscription, Console, gateway) or keep the main model (Bedrock
   and similar), and a hook cannot tell which.

A model found in the transcript always wins over settings. Claude Code sends no session **list**, so
without one from discovery or `session-models.json` it is built from that baseline
(`core.CatalogSessionFromCurrent`, `SessionFromCatalog: true` in `ClaudeCodeCaps`): one catalog
`session_candidates` name per tier below it plus the model itself, so downshift can descend and
upshift never passes the current model. `"session_candidates": {"claude-code": []}` in the user
catalog turns it off.

## 3. When the baseline is unknown or guessed

Code: `internal/core/selfcheck.go` (R6/R7/R9), `claudecode.go` (`Decision.Baseline.GuessHeld`),
`internal/telemetry/reason_codes.go`.

- **Unknown** (`verdict: UNKNOWN`, `requested_model: unknown`): the session list cannot be inferred,
  so no rewrite unless a known list exists; then only a confident classification writes. Unconfident
  picks are held with `R6_UNCONFIDENT_UNKNOWN_SMALL`, `R9_UNCONFIDENT_UNKNOWN_MID`
  (logged `HELD_UNCONFIDENT_UNKNOWN_MID`) or `R7_UNCONFIDENT_UNKNOWN_FRONTIER`. Until 2026-10-15
  `Explore` and `Plan` had no inferred baseline, so every such spawn was held this way.
- **Guessed** (`BASELINE_FROM_SETTINGS`): the session may really be on another model (`--model`,
  `/model`). A downshift is written only when the target is the **small** tier, the one cheaper than
  anything the session can run. A downshift to mid is held with `Decision.Baseline.GuessHeld`, reason code
  `HELD_GUESSED_BASELINE`. The only `UPSHIFT` verdict is an operator tier-floor lift (Downshift never
  raises a tier on its own); on a guessed baseline it lifts only to the floored tier, never to the
  strongest available (`core/session_select.go`, `Decision.Baseline.Guessed`; legacy upshift removed 2026-10-31).
- `Explore` under Fable stays unknown even with a settings hit (point 6 above).

The cost record follows the same honesty: a `usage` row whose decision was `BASELINE_FROM_SETTINGS` is
counted as "inferred baseline" in `downshift stats`, and an unknown requested model counts the spawn
as unrouted with zero savings (`internal/telemetry/usage.go`).

## 4. `/model` and `--model` are invisible to a hook

Code: `configuredmodel.go` (doc comment), `reason_codes.go` (`ReasonBaselineFromSettings`).

Neither the `--model` flag nor a mid-session `/model` reaches the PreToolUse payload or the settings
files. While the transcript has an assistant message the adapter reads the real resolved model, so
these switches are picked up after the first response. Before that, or whenever the transcript is
empty (section 5), the settings fallback can name the wrong baseline. Consequences: a session started
with `--model haiku` could be "downshifted" from a guessed Sonnet, which is why only the small tier is
written on a guess; `downshift session --why` shows `BASELINE_FROM_SETTINGS` on such decisions.
When present in the decision record, the same command also shows its permission mode and recorded
quota status. These are observations for diagnosis; the mode label does not change routing.

## 5. `claude -p` and parallel Agent calls in the first message

Code: `sessionmodel.go` (`transcriptWaitTotal`), `configuredmodel.go`; e2e
`cmd/downshift/baseline_from_settings_e2e_test.go`, `parallel_link_e2e_test.go`.

Claude Code writes an assistant message to the transcript only when it has finished streaming. In a
headless run whose first action is to delegate, one message carries several `Agent` calls, and the
hooks of all but the last see an empty transcript. Measured 2026-10-15 in 3 runs of 5 parallel calls:
4 of 5 spawns per run had no baseline and were held (`HELD_UNCONFIDENT_UNKNOWN_MID`). The 400 ms
re-read does not help (the message closes after all tool-use blocks), and `SessionStart` does not
carry the model under `claude -p` (checked 2026-10-15). The adapter therefore falls back to the
configured model (section 2, step 5) and marks the decision `BASELINE_FROM_SETTINGS`; each decision
still records `tool_use_hash` from `tool_use_id`, so PostToolUse and SubagentStop link to the right
spawn even when five launch at once. A missing transcript file (no persistence) is not waited for.

## 6. Emitted, honored, actually used: three facts, three observations

Code: `claudecode.go` (`Handle`), `posttooluse.go`, `subagentstop.go`, `internal/childend/record.go`,
`internal/telemetry/honor.go`.

| Fact | Where it is observed | Field |
|---|---|---|
| Downshift wrote a model | PreToolUse response, `hookSpecificOutput.updatedInput.model` | event `outcome: "rewrite_emitted"` |
| Claude Code chose that model for the child | PostToolUse `tool_response.resolvedModel` (async launch payload, no tokens) | `outcome: "resolved"`, `linked_decision`, `rewrite_honored` true/false |
| The API served it and what it consumed | SubagentStop, child transcript at `agent_transcript_path` (last line per message id, `message.model`) | `outcome: "usage"`, `agent_hash`, `input_tokens`, `output_tokens` |

`rewrite_emitted` proves emission only. Claude Code validates `updatedInput.model` against family
names (`StrictModelName: true`), so the adapter writes the catalog `native_name` (`plan.WriteName`,
`haiku`/`sonnet`/`opus`/`fable`) and allows the spawn unchanged when an entry has none; a full id
would fail schema validation and block the spawn. Evidence, 2026-10-06: the first build wrote
`claude-haiku-4-5` and Claude Code refused it; after the fix the hook wrote `haiku` and the child
transcript reported `claude-haiku-4-5-20251001` on all 12 assistant records of one spawn and 4 of a
second, with the parent on Sonnet 5.5
([write-up](../evidence/claude-code-rewrite-honored-2026-10-06.md)). One session, two spawns: not a
rate. On 2026-10-09, 5 of 15 usage records linked to a decision and all 5 matched the native
transcript's tokens and model ([billing audit](../evidence/billing-gap-audit.md)); P3.4 (50 linked
records) stays open. `downshift doctor` prints `rewrite honored: seen: N of M rewrites`.

## 7. What setup writes, what the plugin writes

Code: `cmd/downshift/setup_targets.go`, `setup.go` (`mergeTarget`, `ensureStatusLine`, `--undo`),
`doctor_hooks.go` (`checkMainPolicy`, `pluginEnabled`); plugin `hooks/hooks.json`, `.claude-plugin/`.

`downshift setup claude-code` merges into `~/.claude/settings.json`, never replaces a value you set,
keeps a backup, and prints a diff (`--dry-run` first):

- Hooks: `PreToolUse` and `PostToolUse` with matcher `Agent|Task` (`downshift claude-code`,
  `downshift claude-code-post-tool-use`), `PostToolUseFailure` (same runner), `SubagentStop`
  (`downshift claude-code-subagent-stop`).
- Main-session policy, each key **only when unset**: `model: "opusplan"`,
  and `permissions.defaultMode: "plan"`. `--no-session-policy` skips these two. Claude Code's built-in
  You should know mod (`enabledPlugins["cc-plugin-you-should-know@builtin"]: true`) is **opt-in**:
  each check is a billed background request to the session's model, so `setup` only prints the
  suggestion and writes it with `downshift setup --you-should-know`. An existing value, `false` included, is
  reported by `doctor` as `main session: … (set by you)` with the note on how to switch.
  `autoCompactWindow` is not written by setup; set it yourself if you want it.
- `statusLine`: set only when none exists; someone else's is left alone with a note on chaining
  (`quota import --harness claude-code --source claude-statusline`).
- `--undo` restores the latest backup of each hook file (a marker removes a file setup created).

The plugin (`/plugin install downshift@downshift`, Claude Code 2.1.275+) carries the **same four
hooks** from `hooks/hooks.json`, the `/downshift:status` command and the skills. It writes **no
settings**: no `opusplan`, no plan mode, no You should know, no statusline. Use one channel: a
plugin hook and a settings hook with the same command both run, so every spawn would be routed
twice. `setup` skips the hooks when `enabledPlugins["downshift@downshift"]` is true in the user
settings file (policy and statusline still apply); `doctor` warns `plugin … every spawn is routed
twice` when it finds both, and suggests `downshift setup --undo`. Detection reads the user file only.

Mods (plugins with a hooks module that run inside Claude Code, 2.1.287+) are a third channel. Downshift bundles none (the vendored ones were removed on 2026-10-29). Two are recommended, not bundled: Anthropic's
built-in You should know and the Token Weather sample
([companions](companions.md#claude-code-mods-downshift-recommends)).

## 8. Agents: pinned model versus inherit, and when not to delegate

Code: `sessionmodel.go` (`inheritsSessionModel`), `agents/mechanical-worker.md`,
[subagents.md](subagents.md); measurement
[subagent-routing-ab-2026-10-15.md](../evidence/subagent-routing-ab-2026-10-15.md).

A subagent is routable only when its model is not pinned. Frontmatter `model: opus` (or any alias or
id) is an explicit request: the hook sees it as the requested model and leaves it. No `model`, or
`model: inherit`, means the spawn inherits the session model and the hook may rewrite it from the
task text. `mechanical-worker` ships without a `model` on purpose for that reason; built-ins like
`statusline-setup` (Sonnet) and `claude-code-guide` (Haiku) are pinned and log `unknown`.

Delegation is not free. A child loads its own system prompt, tools and the context you hand it, and
its report is re-read by the parent, so a one-line edit can cost more as a spawn on Haiku than as
two tool calls in the main agent. The 2026-10-15 measurement supports savings only where mechanical
work is split into bounded pieces (b5: 34% of spawns to Haiku, about 12% cheaper, 12/12 accepted in
both arms); on natural delegation it applied 0 of 10 and 3 of 34 downshifts, and the cost gaps there
were agent variation, not routing. `downshift route --harness claude-code --model <id>` shows the decision before
you spawn; write the brief as the bounded task, not the whole context.

## 9. Known limits, dated

- **Free plan**: no real subagents, nothing to route (install.md, plan compatibility).
- **`--model` and `/model` invisible** to a hook; wrong baseline possible on an empty transcript,
  mitigated by writing only the small tier on a guess (2026-10-15).
- **Parallel Agent calls in the first message**: 4 of 5 spawns had no transcript baseline; now
  guessed from settings, not known (2026-10-15).
- **`Explore` under Fable**: baseline left unknown because the provider decides (2026-10-15).
- **Pinned agent types**: no inferred baseline; `claude-code-guide`, `statusline-setup`, custom
  agents with `model` log `unknown`.
- **Workflow-created agents never reach PreToolUse**: only `SubagentStart` fires, with no task text
  or model; they run on the session model. Set `model` on the workflow call instead (2026-10-11,
  [evidence](../evidence/workflow-agents-bypass-pretooluse-2026-10-11.md)).
- **Plan-mode spawn on Opus**: follows from the code path, not observed live (session-models.md).
- **Honored evidence**: one session, two spawns (2026-10-06); linked usage 5 of 15, P3.4 open
  (2026-10-09). `rewrite_honored` is read from `tool_response.resolvedModel`, a harness report, and
  the API model from the child transcript; neither is an invoice.
- **`systemMessage` on PostToolUse and SubagentStop**: documented hook output, not yet seen rendered
  on a live session (2026-10-13). `DOWNSHIFT_QUIET=1` silences it.
- **`PostToolUseFailure` payload**: still unverified (GAP-CHILD-FAILURE, 2026-10-13).
- **Plugin detection** reads `~/.claude/settings.json` only; a plugin enabled from a project's
  `.claude/settings.json` is not seen, so double routing there is not warned about.
- **Per-subagent `autoCompactWindow`** (agent frontmatter, 2.1.296+) is not set by Downshift.
- **Hook payload fields** can change per Claude Code release; re-verify `transcript_path`,
  `permission_mode` and `tool_response.resolvedModel` when a release moves them.

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift
