<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# Can Downshift become a Claude Code mod?

> **Removed 2026-10-29.** The prototype (`mods/downshift-router/`) is gone. This design stays as history.

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

Date: 2026-10-17. Status: design plus an opt-in prototype (`mods/downshift-router/`, Experimental).
Claude Code 2.1.302 on this machine; declarations read from the bundled `claude-code.d.ts` written
by 2.1.302 and from the public copy at
`anthropics/claude-code/main/mods/types/claude-code.d.ts` (written by 2.1.277).

## Short answer

Yes, as an **additional Claude Code adapter**, not as a replacement for the hook binary. A mod
(2.1.287+) can choose a subagent's model in-process at `agent.spawn`, change effort per request at
`turn.step`, and read each agent's token usage at `turn.complete` without parsing transcripts, all
of which the settings `PreToolUse` hook cannot do. Everything that makes Downshift Downshift
(classifier, policy, catalog, session rules, guardrails, telemetry) stays in the Go core, which the
mod calls over the same stdin contract the settings hook uses; the layering rule holds because the
mod only parses an event, runs the shared binary and applies the wire result. Both paths must not run
on the same session: when they do, every spawn is routed twice (observed below).

## 1. What a mod can do that a settings hook cannot

| Capability | Settings hook (today) | Mod | Source |
|---|---|---|---|
| Choose the subagent's model | `updatedInput.model` from a child process, validated against family names | `next({ ...e, model })` in-process at `agent.spawn`; "an alias resolves like the tool's parameter" | d.ts `'agent.spawn'` doc; events reference row `agent.spawn` |
| Know the parent's effective model | Read from the transcript tail (400 ms wait) or guessed from settings (`BASELINE_FROM_SETTINGS`) | `e.parentModel`: "The parent's effective model, what `inherit` resolves to. Pinned" | d.ts `AgentSpawnInput.parentModel` |
| Know the resolved agent type | `tool_input.subagent_type` only when the call named one (`unspecified` otherwise) | `e.subagentType`: "The resolved agent type (`general-purpose`, `Explore`, a plugin's agent, `fork`)" | d.ts; live run 1 below (`unspecified` in the hook event, `general-purpose` in the mod's) |
| Change reasoning effort per request | `reasoning_effort` in `updatedInput`, dropped by Claude Code (harness matrix) | `turn.step`: "`next({ ...e, model })` or `effort` sends another"; `effort?: 'low' \| 'medium' \| 'high' \| 'xhigh' \| 'max' \| number` | d.ts `TurnStepInput` |
| Usage per agent | `SubagentStop` payload → child transcript → tokens per message id (`internal/childend`) | `turn.complete` carries `e.agentId` and `e.usage` ("its real requests' token counts ... and the model of the last that counted") | d.ts `TurnCompleteFields` |
| Know when a spawn is a fork, a teammate, a resume, a workflow agent | Not in the PreToolUse payload | `e.fork`, `e.isTeammate`, `e.resume`, `e.workflow`, `e.background`, `e.parentAgentId` | d.ts `AgentSpawnInput` |
| Draw | Nothing (a `systemMessage` line) | `$.ui.log`, `$.ui.status`, a band above the prompt (`ui.render` on `AbovePrompt`), a pane | mods overview |
| State | Files under `~/.downshift` | `$.store` across sessions, `$.state` in-session | mods API |

## 2. What a mod cannot do

- Run anywhere but Claude Code: Cursor, Codex, Antigravity, KiroCrew keep the hook binary. The
  layering rule therefore makes the mod a fifth adapter, never the place where behaviour lives.
- Draw under `claude -p` or the Agent SDK ("Hooks run: Yes. What the mod draws appears: No"); in a
  WSL Desktop session plugins do not load at all. The prototype writes an optional trace file for
  headless runs for that reason.
- Be sandboxed: "Mods aren't sandboxed ... a process that a mod starts runs outside it". The binary
  runs as the user, as the settings hook does today.
- Set the model of a fork ("Ignored for forks, which always inherit") or of a session that forces one
  model on every subagent ("that outranks a workflow agent's, a hook's included"). A resume
  (`e.resume`) can only be refused. `SendMessage` resuming a subagent does not fire `agent.spawn`.
- Reach the agent definition's own frontmatter `model`: `e.model` is "the Agent tool's `model`
  parameter as given ... undefined lets the agent's own model, then the parent's, decide". The
  adapter's inheriting-type rule (`inheritingAgentTypes`) is still needed.
- Count the binary's time against its own budget in a way that matters: a hook's own code has 10 s,
  but "not counting time inside `next` or a mods API call", and `$.process.run` has 30 s by default.
  The real bound is the binary's own (2 s stdin read, 1.5 s `hookctx.HookBudget`), observed 30 ms.

## 3. The `agent.spawn` event, from the declarations

`AgentSpawnInput` in the 2.1.302 declarations (the public 2.1.277 file has the same fields minus
`isTeammate`, `workflow` and `resume`):

```ts
export type AgentSpawnInput = {
  tool_use_id: string;      // "The Agent tool call this spawn belongs to ... Pinned: the spawn's identity."
  prompt: string;           // "The task the subagent is given: the Agent tool's `prompt` parameter."
  description: string;      // "The Agent tool's short `description` of the task (a few words)."
  subagentType: string;     // "The resolved agent type (`general-purpose`, `Explore`, a plugin's agent, `fork`)."
  provider: Origin;         // "Who provides this agent: the plugin and its tier"
  model?: string;           // "The Agent tool's `model` parameter as given, an alias (`haiku`) or a full id;
                            //  undefined lets the agent's own model, then the parent's, decide."
  parentModel: string;      // "The parent's effective model, what `inherit` resolves to. Pinned"
  parentAgentId?: string;   // "The id of the loop the spawn happens in ... absent from main."
  permissionMode?: string;  // "The parent's permission mode ... which the subagent inherits."
  background: boolean;      // "True when the subagent will run in the background (or remotely)."
  fork: boolean;            // "True for a fork of the parent ... `model` is ignored."
  isTeammate?: true;        // "Present, and true, when the call starts a teammate"
  workflow?: { runId: string; agentIndex: number };  // "Present when a workflow script's `agent()` starts the agent"
  resume?: { agentId: string };                      // "A hook can only refuse it"
  name?: string;            // "Given by the call (`Agent({ name })`)"
  cwd?: string;             // "The directory the subagent runs in when the call set one"
};
```

The event doc: "Fires when the Agent tool, or a workflow script's `agent()` (`e.workflow`), is about
to start an agent, its model not yet resolved. `next(e)` resolves to `{ model }`. Return it,
`next({ ...e, model })`, `{ model }` of your own (an alias resolves like the tool's parameter), or
`{ deny: reason }`." and "A guard that fails is skipped and the subagent starts".

So the event **does** carry the task text (`prompt` and `description`, the two fields the Claude Code
adapter already reads through `hookutil.TaskText`), the requested model (`model`, exactly as the call
gave it, absent when it inherits) and the subagent type (resolved, not as typed). Signals the
classifier uses come from the task text and are all available; nothing in `RawSignals` depends on a
field the event lacks. What the event does **not** carry, compared with the PreToolUse payload the
adapter decodes today: `transcript_path` (the adapter's source of the parent's model when the call has
none), `session_id` (available through `$.session.id()`), `cwd` of the session (through
`$.session.cwd()`), and, as in the PreToolUse payload itself, no session list, metadata or quota
marks. In exchange it carries `parentModel`, which the settings hook has to reconstruct. One
workflow caveat: "A workflow agent's [prompt] is a fixed placeholder on a run that keeps its script
out of SDK events", so the prototype passes workflow agents through.

## 4. Ordering: `agent.spawn` versus the settings `PreToolUse` hook

What the documentation says (`docs/en/plugins/mods/events`, "Where settings hooks run in the order"):
"`PreToolUse` hooks from every other settings file and from plugins' `hooks/hooks.json`: run after
the last mod calls `next`, as part of Claude Code's own behavior." `agent.spawn` is raised by the
Agent tool itself while it runs, after the tool's input (including any `updatedInput` a PreToolUse
hook returned) has settled.

What was verified here, 2026-10-17, two headless runs with both paths installed (the settings hooks
of this machine's `~/.claude/settings.json` plus `--plugin-dir ./mods/downshift-router`,
`DOWNSHIFT_MOD_ROUTER=1`, `DOWNSHIFT_EVENT_LOG` on a scratch file):

| Run | Settings PreToolUse decision | Mod decision | Gap | Same `tool_use_hash` |
|---|---|---|---|---|
| 1, "reply with done" | 12:37:44.360Z, `requested_model: claude-sonnet-5-5` (transcript), `agent_type: unspecified`, OK/NO_SIGNAL | 12:37:44.421Z, `requested_model: unknown`, `agent_type: general-purpose`, HELD_UNCONFIDENT_UNKNOWN_MID | 61 ms | yes (`100d3b13…`) |
| 2, "run ls, first three names" | 12:39:19.084Z (hook finished at .141Z after 173 ms per the debug log) | 12:39:19.236Z (`$.process.run` started .217Z, binary exited after 30 ms; the mod's `agent.spawn` "settled in 187.9ms (worker hop, next() included)") | 152 ms | yes (`9d92cea7…`) |

So: **the settings `PreToolUse` hook runs first, `agent.spawn` second**, and **with both installed
every spawn is routed twice**: two decision events for one `tool_use_id`, one per entry point. The
mod's decision shows the missing-transcript gap of section 3: `requested_model: unknown` where the
hook had `claude-sonnet-5-5`. Run 2's `SubagentStop` printed "usage recorded without matching
decision"; whether the duplicate decision caused that is not established (run 1 had no usage record
at all, as is usual for a child that answers in one step).

Not verified: (a) that `e.model` at `agent.spawn` carries a model the settings hook wrote; neither
task produced a rewrite (`NO_SIGNAL` in both paths), so this follows from the chain order only;
(b) interactive sessions, teammates, workflow agents and `turn.step` effort, none exercised;
(c) that a model the mod writes is honored by the API (`PostToolUse` `resolvedModel`), the same P3
evidence the hook path collected on 2026-10-06 and 2026-10-09 (`docs/guides/claude-code.md`,
section 6) and which the mod has yet to collect.

## 5. Proposed architecture

```
agent.spawn (Claude Code, in-process)
   │  e.prompt, e.description, e.subagentType, e.model?, e.parentModel, e.tool_use_id, ...
   ▼
mods/downshift-router/hooks/register.ts        [adapter: parse, call, apply; no policy]
   │  PreToolUse-shaped JSON on stdin ($.process.run has `stdin`; no new flag is needed)
   ▼
downshift claude-code                           [runHookAdapter + claudecode adapter + core, unchanged]
   │  hookSpecificOutput.updatedInput.model, stderr summary, events.jsonl record
   ▼
next({ ...e, model })   or   next(e)            [fail open; never deny]
```

- **Contract.** The existing stdin contract is enough: `ProcessRunInit.stdin` is "Text written to the
  child's standard input, then closed". A `--payload-from-mod` flag is not needed and would be a
  second entry point to keep in step. The payload is `hook_event_name: "PreToolUse"`,
  `tool_name: "Agent"`, `tool_use_id`, `session_id`, `cwd`, `permission_mode`, `tool_input {prompt,
  description, subagent_type, model?, name?, run_in_background, cwd?}`, plus `parent_model`.
- **Shared core untouched** in phase 1. Phase 2a lands two small, harness-free additions in the
  layers the rule names, not in the mod:
  1. `parent_model` (optional payload field, Claude Code adapter): used as the baseline **only**
     under the adapter's existing inheriting-type rule, before the transcript tail. Same rule, one
     more source, no new policy. The `Baseline.FromSettings` guess path becomes rarer when the mod
     is the live path.
  2. `entry` on the telemetry event (`hook` by default, `mod` when the payload says so through an
     optional `EntryIdentifier()` method read by `runHookAdapter`, like `ProjectDir()`), and
     `HarnessCapabilities.HookEntries` as data (`["settings-hook", "mod"]` for Claude Code) so
     `stats`, `report` and `doctor` can tell the two entry points apart and warn when one
     `tool_use_hash` carries two decisions. Prompts, tasks and paths stay out of the log as before.
- **Enablement** is a `userConfig` boolean (`enabled`, default `false`) or `DOWNSHIFT_MOD_ROUTER=1`.
  `userConfig` is the supported mod surface (`/plugin configure`, `pluginConfigs` in settings, a
  reload on change); the env variable exists because a `--plugin-dir` plugin under `claude -p` has no
  stored options. The option wins when set.

## 6. Risks

| Risk | Where it bites | Mitigation |
|---|---|---|
| Double routing | Both the settings/plugin `PreToolUse` hook and the mod enabled: two decisions per spawn (section 4), possibly two different rewrites, telemetry double-counts rewrites | Off by default; README says so; phase 2 `entry` field + `doctor` warning on duplicate `tool_use_hash`; phase 3 removes the `PreToolUse` entry from the plugin's `hooks.json` when the mod is the route |
| Time | Mod hook budget 10 s (own code only); `$.process.run` 30 s default; binary 2 s read + 1.5 s classification; the engine wrapped the whole `agent.spawn` in about 190 ms | Prototype caps the run at 8 s; the binary's own deadline is the real bound (30 ms observed) |
| Failure modes | Thrown error → hook skipped, subagent starts; non-zero exit, timeout, non-JSON → the mod returns `next(e)`; never `{ deny }` | All tested with the mock engine; fail open at every step |
| Model name validation | A full id may be refused where an alias is expected | The binary already writes the catalog `native_name` (`plan.WriteName`), the same value the hook path proved on 2026-10-06 |
| Wrong baseline | Without `parent_model` the mod's decision is `unknown` and held; with it, naive use would override a custom agent's pinned model | Keep the inheriting-type rule in the adapter (phase 2); the prototype deliberately changes no baseline logic |
| Surface drift | "EARLY ACCESS: this surface may change between releases without notice" | Pin the fields used to the quoted declarations; `claude plugin validate` and the tests run in CI |
| Not sandboxed | The binary runs as the user | Same as today's hook; no new reach |

## 7. Migration path

1. **Phase 1 (done, 2026-10-17).** Opt-in prototype under `mods/downshift-router/`, its own manifest, not
   loaded by the root plugin. `claude plugin validate` and `claude plugin test` pass; two live
   headless runs show the ordering and the double-routing shape. No Go change.
2. **Phase 2a (done, this change).** `parent_model` and `entry` as in section 5: the Claude Code
   adapter reads `parent_model` under its inheriting-type rule (before the transcript); the shared
   runner records `entry` (`hook` or `mod`) on every decision; `HarnessCapabilities.HookEntries`
   lists both paths for Claude Code; `stats` counts by entry and `doctor` warns when one
   `tool_use_hash` carries two decisions. The mod always sends `entry: "mod"`.
3. **Phase 2b (done, this change).** A `turn.complete` hook in the mod accumulates `e.usage`
   per `e.agentId` and, once `$.agent.list()` shows the loop finished, records the totals through
   `downshift end claude-code` with inline usage (same ChildEnd recorder `SubagentStop` feeds; the
   Claude Code adapter prefers inline `usage` over a transcript path). Dedup records each agent's
   usage once when the settings `SubagentStop` hook and the mod overlap. A `turn.step`
   hook stores `reasoning_effort` from the mod spawn's binary stdout (the adapter emits it in
   `updatedInput` only when `entry` is `mod`) and returns `next({ ...e, effort })` per subagent
   request. Still open: live `PostToolUse` `resolvedModel` evidence for mod-written models (the
   honored count the hook path already reports).
4. **Phase 3.** Once honored is observed for mod writes at the same bar as the hook path (P3.4,
   fifty linked records), make the mod the Claude Code route for 2.1.287+: the root plugin's
   `hooks.json` keeps `PostToolUse`, `PostToolUseFailure`, `SubagentStop` and drops `PreToolUse`
   when the mod is enabled; `setup` keeps writing settings hooks for older Claude Code and for every
   other harness. The harness matrix gets a "mod" column for Claude Code.

## 8. Recommendation

Adopt the mod as Claude Code's second adapter and keep the binary as the product. The owner's
question has a positive answer with two conditions: the mod never holds policy (it is twenty lines
of parse, run, apply around the shared binary), and the two entry points are never both live on a
session. The gain is concrete and specific to Claude Code: the parent's model and the resolved agent
type arrive as facts instead of transcript reads and guesses, effort becomes settable, and usage per
agent arrives as an event. The cost is a second packaging to test on every release of an early-access
surface. Phase 1 shipped off by default; phase 2a (`entry` + `parent_model`) is on `main` so the two
paths are comparable in `stats` and `doctor`; phase 2b (`turn.complete` usage + `turn.step`
effort) is in; decide phase 3 on honored evidence, not on the prototype alone.

## Appendix: evidence

Validation and tests (2026-10-17, Claude Code 2.1.302):

```
$ claude plugin validate mods/downshift-router
  > ./register.ts hooks: agent.spawn
  > ./register.ts gating hook without .catch: agent.spawn
  > ./register.ts calls: $.env.get (via appendTrace, isEnabled), $.fs.read (via appendTrace), $.fs.write (via appendTrace), $.process.run, $.session.cwd, $.session.id, $.ui.log
  > ./register.ts env writes: nothing
  > ./register.ts env reads: DOWNSHIFT_MOD_ROUTER, DOWNSHIFT_MOD_ROUTER_LOG
√ Validation passed

$ claude plugin test mods/downshift-router
 14 pass
 0 fail
Ran 14 tests across 1 file.
```

"gating hook without `.catch`" is intended: a failing `agent.spawn` hook is skipped and the subagent
starts, which is the fail-open behaviour Downshift wants; a `.catch` would only matter for a guard
that must fail closed. `tsc -p` against the bundled declarations: no errors.

Live run command (both paths installed; each run finished in 12 s and 18 s, well under $0.10):

```
DOWNSHIFT_MOD_ROUTER=1 DOWNSHIFT_MOD_ROUTER_LOG=<scratch>/router.jsonl DOWNSHIFT_EVENT_LOG=<scratch>/events.jsonl \
claude -p --plugin-dir ./mods/downshift-router --model sonnet --max-budget-usd 1 --allowedTools "Agent" \
  --output-format stream-json --verbose --debug-file <scratch>/debug.log \
  "Spawn one Agent subagent that replies with the word done"
```

Mod trace line, run 1: `{"event":"agent.spawn","tool_use_id":"toolu_016vk2…","subagent_type":"general-purpose","requested_model":null,"parent_model":"claude-sonnet-5-5","written_model":null,"elapsed_ms":33,"note":"no model in hook output"}`.
Debug log, run 2: `Hooks: PreToolUse:Agent [/root/.local/bin/downshift claude-code] finished with status 0 (173ms)` at 12:39:19.141Z, then `$.process.run (downshift-router): downshift with 1 args` at .217Z, `exited 0 in 30ms`, `hooks module downshift-router@inline agent.spawn settled in 187.9ms`. Under `-p` the `$.ui.log` line appeared only in the debug log, as the overview's table says.
