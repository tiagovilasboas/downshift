<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# Spike: `downshift-progress`, a Downshift-owned replacement for `mods/savvy-progress`

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

Read-only spike on `origin/main` (b1e4ed21), worktree
a scratch worktree of origin/main.
Nothing committed. Measured on a copy of `~/.downshift` logs (`DOWNSHIFT_STATE_DIR`) with a binary built from the
worktree. Baseline: `claude plugin test mods/savvy-progress` = 46 pass / 0 fail; `claude plugin validate` passes
(only an ICON_MISSING advisory).

This note describes **behaviour**, not code. The new mod is a clean-room reimplementation (see section 5.4).

---

> **Owner decisions applied (2026-10-24):** finished rows expire after **10 s** (owner, 2026-10-21:
> "somente exclusão automática dos finishs por 10 segundos"); **no buttons** (owner asked to remove the
> returned savvy-progress buttons); languages **en + pt-BR**; band gate, crab sprite and dotted grid
> dropped. The tile prints the `report` block verbatim (owner, 2026-10-23).

## 0. Decisions the owner must confirm before M2/M4/M5 start

| # | Question | Evidence | Recommendation |
|---|---|---|---|
| Q1 | Finished-row expiry: 10 s or 30 s? | #333 (2026-10-20) set 10 s; #411 (2026-10-21) changed it to 30 s and the code, README, tests and spawn-state doc say 30 s today. CHANGELOG 1.2.8 still says "10 s". The brief says 10 s. | One constant `FINISHED_SHOW_MS`, default **10 s** per the brief; owner confirms. |
| Q2 | "No buttons" | The only recorded decision is 2026-10-21: `downshift-dock` (Files/Agents buttons) left the marketplace. #342 (owner) made the band's `✕` work. Today's mod has `×N` crew button, `✕`, Expand/Collapse, Finished fold. | New mod draws **no `Button` at all**: the pane opens by command; finished rows expire by themselves (so `✕` is not needed); the engine's own `[-]` / ctrl+x ctrl+a collapses the band. If the owner wants `✕` kept, it is one `Button role="dismiss"` in the band (1 point). |
| Q3 | Is the band gated on "Downshift is working in this session"? | Today: derived band only if `session --panel` lists >= 1 decided spawn; otherwise it "draws exactly what upstream draws". | Drop the gate: this mod is Downshift's own, so the band follows live subagents (and a registered flow). Without the binary it still shows agents, just without Downshift figures. |
| Q4 | Languages | en/ru/he inherited from upstream; repo docs are en + pt-BR (`docs/pt`). Hebrew needs RTL layout code. | v1 ships **en** and **pt-BR** behind a string table; ru/he DROP (translations are upstream's; cannot be copied). |
| Q5 | Pixel crab, dotted grid, shimmer on the desktop Svg bar | Owner 2026-10-19: one monochrome crab. Decoration, ~250 lines of SVG code. | DROP in v1: text bar (`█░`) on every surface. Revisit as an optional, original follow-up. |

---

## 1. Feature inventory of `mods/savvy-progress` (1.2.13)

Sizes: `hooks/register.tsx` 1481 lines (limit ~300 per file), 5 test files 620 lines (46 tests),
`downshift-local.patch` 920 lines, README 153 lines. `plugin.json`: userConfig `language`, `autoOpen`; `types/index.d.ts`:
state keys `savvy-progress.{flow,agents,panel,now}`. Hooks: `session.start`, `command.run{agents-info}`,
`tool.call{progress|step|Agent}`, `agent.spawn`, `turn.step`, `turn.complete`, `ui.render{Pane savvy-agents|AbovePrompt}`.

Tags: KEEP / DROP / CHANGE. "Owner" marks something this repo's history records as an owner request or decision.

### 1.1 Band above the prompt (`AbovePrompt`)

| Feature | Tag | Reason |
|---|---|---|
| Flow row: title, phase, `done/total`, percent, bar (registered by the `progress` tool) | KEEP | Core of savvy-flow; the main session reports it. |
| Band that follows live subagents with no registered flow: title `Subagents`, `N running · M done`, bar over the current batch, finished batch lingers | KEEP, CHANGE | Owner-requested (2026-10-19, README "band fallback"). Lingers by the same expiry constant as the pane (Q1). Gate on "Downshift active" dropped (Q3). `lastBatch` logic stays but is derived from the roster, not re-derived per render. |
| One short line `Downshift · N model · M effort · K held` (older: `N switched`; `on` when tokens moved but nothing switched) | KEEP, CHANGE | Owner 2026-10-20 ("say less"). The string is composed in Go (`summary`, G1a); the mod prints it verbatim. |
| `×N` crew button that toggles the pane | DROP | Q2 (no buttons). Opening is `/agents-info`. |
| `✕` dismiss (and `dismissedAt` bookkeeping) | DROP (CHANGE if Q2 says keep) | Owner #342 fixed it, but auto-expiry removes the need. |
| Desktop Svg row with noise-shimmer bar and crab; terminal text bar | CHANGE | One text-bar code path on every surface (Q5). |
| `hasSurvey` yield; compose with other mods' bands via `next(e)` | KEEP | API etiquette; engine placeholder must not be nested (R5). |
| Brand colours: running = white `#f5f7fa`, done = green `#22c55e`, pill ink `#0a0a0a`, tier grey ramp | KEEP, CHANGE | Owner 2026-10-19 "black first". Tokens read from one `shared/theme` file mirroring `docs/brand/downshift-brand-tokens.json`. |
| Tier words `small / medium / frontier` | KEEP | Owner 2026-10-19; same words as the site. |

### 1.2 Agents pane (`Pane` id `savvy-agents`, `/agents-info`, `/agents-info open`)

| Feature | Tag | Reason |
|---|---|---|
| `/agents-info` toggles, `/agents-info open` brings forward; reply text "Agents panel opened/closed" | KEEP, CHANGE | Pane id becomes `downshift-agents`. Command name: see section 5.1. |
| Auto-open: for `savvy-*` workers or a plan with tasks; `autoOpen: any` option | CHANGE | Keep the option, default `off`/`flow` (opening a pane unasked is intrusive); only when `e.viewport.isFullscreen`-style docking exists (reference.md: open unasked only where it would be a sidebar). |
| Header tiles: `Tokens saved · <window>` (value = `report.tokens_saved_estimate` through the token formatter, window from `report.scope`), `Tokens`, `Time` | KEEP, CHANGE | Owner 2026-10-21 (one final metric) and 2026-10-23 ("mods show the report's numbers", #502). Window label and short value come from Go (G1b). `Tokens` (sum of the agents' tokens) and `Time` stay: they are observations of this session, not accounting. |
| Older tiles `Downshifted`, `Compactor ↓` (1.2.5 to 1.2.11) | DROP (already gone in 1.2.12) | Superseded by the single estimate. Optional: one dim detail line printing `report.display.compactor` and `report.display.routing` verbatim (both already pre-formatted by `accounting.Display`). |
| Running / Finished / Planned sections with counts; Finished fold button; Expand/Collapse | CHANGE | Sections stay; **no buttons**: no fold, no compact mode (Q2). |
| Finished rows auto-expire after N s from first seen finished, section count = rows shown, redraw on the clock tick with no other atom changing | KEEP | Owner (#333, #411); Q1 for the value. Must be a pure function of `(rows, now)` (testable) with a 1 s timer that exists only while a row is fading. |
| Per-spawn row: description, tier word, model (+effort), status word, `↓ <tokens>` when tokens moved to a smaller model, `round N` for repeated descriptions | KEEP, CHANGE | Owner 2026-10-20. `↓` value is `spawns[].saved_total` formatted by Go (G1a); today the mod adds `work_saved + cache_saved` (the last arithmetic the owner wanted gone). Status comes from `spawns[].state`, not by parsing the label text. |
| Step progress bar per worker (`stepDone/stepTotal`, note) | KEEP | Needed for `step` tool (1.3). |
| Context bar `ctx N%` using a hard-coded window table (`haiku-4|claude-3` -> 200k else 1M) | DROP | Model knowledge in the mod; the catalog has no context-window field. Show absolute context tokens only (no percent). |
| Per-agent and per-flow dollar estimate, `PRICES` table, `costOf`, `cost.test.ts` (gated) | DROP | Owner 2026-10-19: tokens only, no dollars. Pricing belongs to the catalog. |
| Planned (not started) tasks matched to runs by normalised description; `after` dependencies shown | KEEP | Part of the flow contract; match logic moves to `flow-progress`. |
| Compact one-line mode, RTL layout, dotted-grid backdrop, crab sprite with walking animation, role costumes | DROP | Q4, Q5. |
| i18n en / ru / he (+ language detection from option, Claude Code `language`, env) | CHANGE | en + pt-BR, same detection chain (Q4). |
| Panel on every surface (`Pane` is raised on all) | KEEP | Text/Box only, so one tree works on terminal, desktop, vscode, mobile. |

### 1.3 Tools registered for savvy-flow (`mcp__savvy-progress__progress`, `...__step`)

| Feature | Tag | Reason |
|---|---|---|
| `progress` tool (main session only; a call from a subagent returns "ignored"): `title,total,done,phase,finished,tasks[{title,tier,after}]`, fields left out keep previous values, bounds (200 tasks, 120-char title, 20 `after`) | KEEP | Used by the "forca total" workflow and savvy-flow. Bounds are a safety property; keep as constants + tests (`caller.test.ts`, `register.test.ts` equivalents). |
| `step` tool (worker reports `done,total,note`; matched to its panel row by `agentId`; `isDeferred: false`) | KEEP | Same. |
| Prompt injection: `agent.spawn` appends a "call the step tool first" line to the prompt of every `savvy-*` worker | CHANGE | Mutating subagent prompts is the most invasive thing the mod does and it keys on upstream's `savvy-` type prefix. Make it opt-in (`stepBrief: off | named | any`, default `off`); the brief text is original and names the new tool. |
| `tool.call{Agent}` safety net that bumps `flow.running` for `savvy-*` | DROP | Redundant once the roster is reconciled with `$.agent.list()`. |
| Flow reset when a new titled flow starts (agents still running stay) | KEEP | Behaviour covered by existing tests. |

### 1.4 Downshift integration (`downshift session --panel`)

| Feature | Tag | Reason |
|---|---|---|
| `dsRefresh`: run `downshift session --panel --session=<id>` (2 s timeout, 3 s TTL on render, forced at spawn and at every subagent turn end) | CHANGE | See section 2.3 for cadence; single-flight, no poll from render. |
| `dsKeyOf`: sha256(`harness-downshift-session-v1:` + tool_use_id) to join a row with `spawns[].tool_use` | KEEP, CHANGE | The one duplicated recipe; isolated in `shared/downshift/key.ts` with a golden vector shared with the Go test (R3). |
| No session id => session figures withheld (never show another session's number) | KEEP | Safety property, has a test. |
| Fail-open: no binary / bad JSON / timeout => draw the agents without Downshift figures | KEEP | Owner invariant "hooks fail open". |
| Older-binary fallbacks (`report` absent => top-level `tokens_saved_estimate` with `~`; `effort_changed` absent => `N switched`) | DROP | New mod declares a minimum binary (the release carrying G1); an older binary draws no Downshift figures. `downshift doctor` gets one check (M7). |
| Adds `work_saved + cache_saved` for the row `↓` | DROP | Replaced by `saved_total` (G1a). |

### 1.5 Other

| Feature | Tag | Reason |
|---|---|---|
| Theme / look | KEEP | `shared/theme`. |
| 1 s `$.clock.every` tick that runs only while an agent runs or a row is fading | KEEP, CHANGE | Same intent; the timer is started by the roster when the first visible row appears and cancelled when none is. |
| Option `language`, `autoOpen` | KEEP, CHANGE | Adds `stepBrief`, `band` (`auto/on/off`, see 5.1). |

---

## 2. Data contract

### 2.1 What the mod reads

Exactly one command, run through `$.process.run(argv, { timeoutMs })` (no shell):

```
downshift session --panel --session=<claude session id>      # $.session.id(), else env CLAUDE_CODE_SESSION_ID
```

Fields used today (all exist on `origin/main`; `docs/reference/spawn-state.md`):

| JSON path | Used for |
|---|---|
| `session` (8 hex) | sanity check that the answer is for the asked session |
| `switched, model_switched, effort_changed, held` | summary line (today composed in the mod) |
| `spawns[].tool_use` (sha256 key) | join to a roster row |
| `spawns[].state` (`switched|held|kept|control|paused|blocked`), `.label`, `.from_model`, `.to_model`, `.observed_model` | status word and model line |
| `spawns[].tokens.work_saved`, `.cache_saved` | row `↓` (to be replaced) |
| `report.tokens_saved_estimate`, `report.scope.{kind,days}` | header tile |
| `report.compactor`, `report.display.{tokens_saved,routing,compactor}` | optional detail line |
| `tokens_saved_estimate`, `compactor` (top level) | legacy; not read by the new mod |

Not read: `tokens`, `tokens_line`, `by_agent_type`, `spawns[].tokens_label`.

The payload is ~22 KB on this machine (50 children in the latest session), most of it `by_agent_type` and
per-spawn `tokens` blocks the mod discards.

### 2.2 Measurements (`downshift session --panel`, binary built from the worktree)

State dir copies: x1 = real logs (events.jsonl 396 KB / 503 lines, exec-events.jsonl 195 KB / 1,207 lines);
x10 and x50 = the same files concatenated 10 and 50 times (synthetic scale-up). 7 runs each, median.

| Logs | Size | `session --panel` | `session --tree` (tail read only) |
|---|---|---|---|
| x1 | 0.6 MB | **28 ms** | 27 ms |
| x10 | 5.6 MB | **78 ms** | 37 ms |
| x50 | 28 MB | **254 ms** | 27 ms |

Reading: process start is ~26 ms. `--panel` cost is linear in log size, about **8 ms per MB** of events + exec log,
because `sessionpanel.go` reads the tail (2 MiB) for the session view **and then reads both whole logs** for
`accounting.Compute(... ScopeDays(30))`. `--tree` shows the tail-only path stays at ~27 ms at any size.
Log rotation is at 100 MB (`DefaultRotationPolicy`), so the extrapolated worst case is **~0.8 to 1 s per call**, against the
mod's 2 s timeout and its calls on render, at every spawn and at every subagent end. At today's size the cost is negligible; the
design must not depend on it staying that way.

### 2.3 Cadence (proposed) and Go-side additions

Fast path (session only, constant ~27 ms) vs full path (adds the report):

| Trigger | Call | Why |
|---|---|---|
| `session.start` | full | header tile has a value from the first draw |
| `agent.spawn` | fast (after the Downshift PreToolUse hook has logged the decision; the spawn hook runs during the tool, so the row exists) | the row label |
| subagent `turn.complete` | fast, then one retry via `$.clock.after(1500)` | tokens are written by the SubagentStop hook, which may run after `turn.complete` (R2) |
| pane open, or every 60 s while the pane or band shows a running row | full | tile figure |
| render | **never** (draw from the last result) | removes the 3 s TTL poll from the render path |

Single-flight: one call in flight, later triggers coalesce. Timeout 2 s, fail-open.

Go additions that make the mod arithmetic- and parsing-free (all in shared layers; the mod is Claude Code only, so
note this in `docs/reference/harness-matrix.md` per layering rule 5; every harness's `--panel` consumer gets them):

| ID | Change | Where | Replaces in the mod |
|---|---|---|---|
| G1a | `spawns[].saved_total` (int64, work+cache saved) and `spawns[].saved_short` (`"58k"`); top-level `summary` (`"Downshift · 2 model · 1 effort · 1 held"`, `""` when nothing true to say) | `internal/telemetry/panel.go` (251 lines; keep under 300) | the `+` of `work_saved + cache_saved`, `dsStatus`, `dsSummary`, the older-binary branches |
| G1b | `report.display.tokens_saved_short` (`"74k"`) and `report.scope.label` (`"last 30 days"`, already `Scope.Label()`) | `internal/accounting/compute.go` (170 lines) | the mod's token formatter for the tile, the scope-word table |
| G2 | `session --panel --no-report`: session view only (tail read), omits `report`/`compactor`/`tokens_saved_estimate`; also add `--brief` that drops `tokens`, `by_agent_type`, `tokens_line`, `tokens_label` (payload ~22 KB -> ~3 KB) | `cmd/downshift/session.go` (198), `sessionpanel.go` (56) | nothing; makes the fast path possible |
| G3 (optional) | stat-validated cache of the report block (`panel-cache.json` keyed by events/exec size+mtime; recompute on mismatch) | `internal/accounting/cache.go` (new), `sessionpanel.go` | full-path cost at 100 MB; only if measurements after G2 still justify it |
| G4 (optional, only if R3 fails live) | `--tool-use-id=<id>` repeatable, answered with `spawns[].tool_use_id_echo` | session.go | the sha256 recipe in the mod |

The mod's remaining computations, all presentation of its own observations and none of them Downshift accounting:
elapsed time (`now - startedAt`), the sum of the four usage fields of a `turn.step` into "tokens used / context tokens"
(`usageTotal`, one function), and counts of rows. State this limit in `shared/format` so a reviewer can reject any other `+`.

Optional later (not in the plan): `downshift session --panel --follow` emitting NDJSON on log change, consumed with
`$.process.spawn`, would replace polling. Needs a file watcher on every OS (AGENTS.md: OS-agnostic), so it is out of scope.

---

## 3. API mapping (KEEP/CHANGE features)

`claude-code.d.ts` facts used: `AbovePrompt` is raised on **terminal and desktop only**; `Pane` on **every attached surface**;
`$.process` is documented **"CLI only"**; `$.agent.list()` returns `{id, description, type, status, parentId...}`; `agent.spawn` returns
`agentId`; `turn.step`/`turn.complete` carry `agentId` and `usage`; `$.tool.register` names the tool `mcp__<plugin>__<name>`.

| Feature | API piece | Events / calls | terminal | desktop | vscode / mobile |
|---|---|---|---|---|---|
| Band (flow row, follows subagents, Downshift line) | `on('ui.render', { component: 'AbovePrompt' })`; `Box`, `Text` from `$.ui.resolve(e)`; props `hasSurvey`, `isWorking`, `bodyColumns`; `next(e)` for other mods' bands | reads atoms `flow`, `roster`, `now`, `downshift` | text bar | same tree (Svg not needed) | not raised; nothing to do |
| Agents pane | `on('ui.render', { component: 'Pane', requestId: 'downshift-agents' })`; `$.ui.open({id,title})`, `$.ui.close`, `$.ui.panes()` | `/agents-info` via `$.command.register` + `on('command.run', { command })` | docks in fullscreen, else inline; size to `e.props.bodyColumns` | docks | draws the same tree |
| Roster | `agent.spawn` (`next(e)` first, then `started.agentId`, `e.tool_use_id`, `e.description`, `e.subagentType`), `turn.step` (async generator, `yield* next(e)`, `result.usage`), `turn.complete` (`e.agentId`, `e.reason === 'answer'`), `$.agent.list()` to reconcile status | `update($, roster, ...)` | | | |
| Finished expiry | pure `visibleRows(rows, nowMs)`; `$.clock.after`/`$.clock.every` timer owned by the roster while a row is fading; redraw because the render reads the `now` atom | | | | |
| `progress`/`step` tools | `$.tool.register` in `session.start`; `on('tool.call', { tool: 'mcp__downshift-progress__progress' })`; `e.agentId` tells main session from worker; `isDeferred:false` for `step` | | | | |
| Step brief (opt-in) | `agent.spawn` rewrite of `e.prompt` through `next({ ...e, prompt })` | option `stepBrief` | | | |
| Downshift data | `$.process.run(['downshift','session','--panel',...], {timeoutMs:2000})`, `$.session.id()`, `$.env.get`, `$.clock.after/now` | `session.start`, `agent.spawn`, `turn.complete` | | | **no `$.process` on remote surfaces: figures absent, rows still drawn (R1)** |
| Language | `options.language`, `$.settings.read('language')`, `$.env.get('LANG')` | | | | |
| State | `atom/read/update` from `claude-code`, keys under plugin `downshift-progress`; no `$.store` (nothing persists across sessions) | | | | |

Tests use `claude plugin test` with the mock engine; every UI test loops `['terminal','desktop'] as const` (band) and all four
surfaces (pane), as `pane.test.tsx` in the skill examples does.

---

## 4. Architecture

Screaming Architecture: folders name the domain. The module that `hooks/hooks.json` names is a **router only**.
Everything is under `hooks/` because that is where the savvy-progress suite, the skill example and `claude plugin test` are
known to work; a layout with `features/` at the plugin root is a verify-first item (R4).

```
mods/downshift-progress/
  .claude-plugin/plugin.json          name downshift-progress, version 0.1.0, license Apache-2.0, userConfig
  hooks/hooks.json                    { "modules": ["./register.tsx"] }
  hooks/register.tsx                  <= 60 lines: calls each feature's register(on, options); no logic
  types/index.d.ts                    state contract (PluginState['downshift-progress'])
  tsconfig.json                       copy of downshift-dock's
  README.md
  hooks/features/
    roster/        spawn.ts (agent.spawn)  steps.ts (turn.step/complete)  expiry.ts (visibleRows, pure)
                   reconcile.ts ($.agent.list)  ticker.ts  index.ts  + roster.test.ts expiry.test.ts
    flow-progress/ flow.ts (merge/reset/bounds)  tools.ts (progress, step)  brief.ts (opt-in prompt line)
                   index.ts  + flow.test.ts tools.test.ts caller.test.ts
    band/          Band.tsx  bar.ts (text bar)  derive.ts (flow or live batch)  index.ts  + band.test.tsx
    agents-pane/   Pane.tsx  Row.tsx  Header.tsx  Sections.tsx  command.ts  index.ts  + pane.test.tsx
    downshift-data/ refresh.ts (single flight + cadence)  index.ts   (the only caller of the client)
  hooks/shared/
    downshift/     client.ts ($.process.run)  parse.ts (defensive, no arithmetic)  key.ts (sha256 recipe)  types.ts
                   + parse.test.ts key.test.ts (golden vector)
    format/        tokens.ts  time.ts  (usageTotal documented as the one allowed sum)
    i18n/          en.ts  pt-BR.ts  lang.ts  strings.ts (type)
    theme/         tokens.ts (colours, tier ramp, tier words)
```

Dependency rule inside the mod: `features/*` import `shared/*` and the `roster`/`flow`/`downshift-data` read APIs
(atoms + selectors), never each other's components; `shared` imports nothing from `features`; no cycles. `band` and
`agents-pane` are leaf renderers. File budget: every file < 300 lines (largest expected: `Pane.tsx` ~150, `flow.ts` ~120,
`refresh.ts` ~100). A tiny script in M0 (`wc -l`) is the sensor; no catch-all `utils/helpers`.

State contract (`types/index.d.ts`, declared via `interface PluginState`):

```ts
'downshift-progress': {
  flow: Flow | null
  roster: AgentRun[]            // id, agentId, type, description, model, effort?, status, startedAt, endedAt?,
                                // contextTokens, tokens, steps, round, stepDone?, stepTotal?, stepNote?
  downshift: DownshiftView | null   // parsed panel answer: strings and integers copied through, nothing derived
  now: number                   // clock tick atom read by renderers so the engine redraws on expiry
  seenFinishedAt: Record<string, number>   // for expiry; atom, not a module Map, so hot reload keeps it
}
```

(No `costUsd`, no `contextMax`, no panel `isCompact`/`isDoneCollapsed`.) Per-feature tests:

| Feature | Tests (each runs through `claude plugin test`) |
|---|---|
| roster | spawn adds a running row; usage steps accumulate; `turn.complete` marks done/failed; expiry boundary at N-1 / N ms; reconcile with `$.agent.list`; row `round` for repeated descriptions |
| flow-progress | merge keeps omitted fields; new titled flow resets; bounds (201 tasks, 121-char title); subagent call ignored; `step` matches `agentId`; step brief only when option on |
| downshift-data / shared/downshift | parse defensive (non-numeric -> absent); missing binary / bad JSON / timeout -> null, draws rows; no session id -> figures withheld; single flight; golden sha256 vector equals the one in the Go test |
| band | flow row; live-subagent fallback; auto-hide when everything expired; `hasSurvey` yields; text-only on terminal and desktop |
| agents-pane | all four surfaces; header prints Go strings verbatim; row `↓` from `saved_short`; no Button anywhere (`find({type:'Button'})` is null); finished section disappears on tick with no other atom changing |

---

## 5. Migration

### 5.1 Coexistence during parity

- Plugin name `downshift-progress`; tools are therefore `mcp__downshift-progress__progress` and `mcp__downshift-progress__step`:
  **no clash** with `mcp__savvy-progress__*`, and state is keyed by plugin name, so no state clash either.
- Slash command: both mods would register `agents-info`. New mod registers **`ds-agents`** during parity. At cutover it registers
  `agents-info` too, but only if `$.command.list()` shows no other plugin owns it (the dock mod already uses `$.command.list()` this way).
- Band: userConfig `band: auto | on | off`. `auto` (default) = draw unless the savvy-progress tool/command is present, so two bars never stack.
  The owner can force `on` to compare side by side.
- Panes have different ids (`savvy-agents` vs `downshift-agents`), so both can be open for the comparison.
- `downshift-dock` (not in the marketplace) names `plugin: 'savvy-progress', pane: 'savvy-agents'`; update in the cutover PR or leave (it is dead code kept for maintainers).
- Callers that name the old tools: the upstream savvy-flow skill and any harness-core instruction. **Grep `~/Github/harness-core` and the
  user's skills for `mcp__savvy-progress` before cutover** (R7); in this repo only `mods/` and docs mention it.

### 5.2 Marketplace and `setup`

- `.claude-plugin/marketplace.json`: add `{ "name": "downshift-progress", "source": "./mods/downshift-progress", "category": "productivity" }` (M6). Remove the `savvy-progress` entry at cutover (M7).
- `cmd/downshift/setup_targets.go:178` `Mods`: `savvy-progress@downshift` -> `downshift-progress@downshift` (M7), plus the `help` text at `setup.go:151`, `setup_plugin_test.go` (3 tests name the id), `llms.txt`, `mods/README.md`, `docs/guides/{install,claude-code}.md`.
- `setup` must not uninstall anything itself (AGENTS.md: assisted flow, write only after a clear yes): if `savvy-progress@downshift` is enabled it prints a note with the exact command `claude plugin uninstall savvy-progress@downshift`, the pattern `ModConflicts` already follows for `cache-tax@claude-code-mods`.
- Cache pitfall (CHANGELOG 1.2.2): Claude Code caches a plugin by `version`; bump `plugin.json` on every behaviour change.

### 5.3 Licence and NOTICE

- `downshift-progress` is original Downshift code, **Apache-2.0**, `plugin.json` `"license": "Apache-2.0"`, every TS/TSX file starts with the same header as `mods/downshift-dock` (`// Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0`) and the "Downshift by Tiago Vilas Boas" attribution line in README. No new NOTICE entry needed for it.
- NOTICE keeps the savvy-progress entry until the vendored directory is deleted (M8), then the entry and the MIT `LICENSE` of that directory go with it. Because nothing is copied, no MIT obligation carries over. (Not a legal opinion; the owner's call.)

### 5.4 Clean-room rule (must hold)

Implementers work from this note and `claude-code.d.ts`, not from `mods/savvy-progress/hooks/register.tsx`, its tests, README prose or patch.
Do not reuse its identifiers, string tables, test names, the `STEP_BRIEF` sentence, the crab/sprite, or the state key names
(`savvy-progress.*`). Reviewer sensor (computational): `git grep -n -F` a short list of distinctive upstream lines
(`STEP_BRIEF`, `savvy-agents`, `dsKeyOf`, `FINISHED_SHOW_MS`, `crabBody`, `TIER_COLOR`) against `mods/downshift-progress` must return nothing. Behaviour parity is proven by
behaviour tests written from section 1, not by porting the old ones.

### 5.5 Retiring savvy-progress

1. Parity release: both in the marketplace, new one default for `setup` only after E1 (live parity evidence) is green.
2. Cutover (M7): `setup` installs the new mod, prints the uninstall note for the old one; marketplace entry removed; docs updated; CHANGELOG entry.
3. One release later (M8): delete `mods/savvy-progress/` (+ `downshift-local.patch`), drop the NOTICE entry, change AGENTS.md "Third-party mods" list to `hooks/cache-tax/`, `mods/filetree/`, fix `scripts/test-ci-classify-changes.sh` lines 61 and 71 (they use savvy paths as examples), remove the roadmap/spike docs references or mark them historical.

---

## 6. Risks and unknowns

| # | Risk / unknown | How to verify | Fallback |
|---|---|---|---|
| R1 | `$.process` is "CLI only" (types, line ~3582). In the desktop app's Code tab, vscode or a remote/SDK host `downshift` may not run, so no figures. Current mod has the same exposure; no evidence it was ever checked on desktop (`docs/evidence/plugin-mods-smoke-2026-10-17.md` predates the panel). | Run the built mod in the Claude desktop app (local session) and in the terminal; log `exitCode` and any rejection | Rows draw without Downshift figures (fail-open); say so in the README support matrix |
| R2 | Race: the SubagentStop hook that writes the child's tokens may run after the mod's `turn.complete`, so the fast call returns the row without tokens. | Live: spawn a trivial agent, compare timestamps (`events.jsonl` write vs mod call) with `claude --debug` | Delayed second call (`$.clock.after`), already in the cadence table |
| R3 | `agent.spawn.tool_use_id` equals the id the Downshift hook hashed (README: "unverified live"), and Web Crypto exists in the mod runtime. Also the sha256 recipe is duplicated between Go and TS. | Live run: check that a `↓` label appears for a switched spawn; golden-vector test in both languages | G4 (`--tool-use-id`), or match by order/time |
| R4 | Files outside `hooks/` (the `features/` dir) may not be loaded by the engine or discovered by `claude plugin test`; `import()` is forbidden; extensions must be `.ts/.tsx`. | M0 spike: 3-file module with one nested import, run `claude plugin validate` and `claude plugin test` | Keep everything under `hooks/` (the plan already does) |
| R5 | AbovePrompt composition: drawing "below" other mods' bands via `next(e)`; the engine's own placeholder (`{type:'engine'}`) must not be nested or the desktop drops the band (a comment in the old mod says so). | Test with a fake second band on both surfaces; live check on desktop | Draw alone and skip `next` |
| R6 | Log growth makes `--panel` slow (8 ms/MB, ~1 s at the 100 MB rotation). Measured only on concatenated copies of one machine's log, not on a real 100 MB log. | Go benchmark with a generated 100 MB log in G2/G3; budget test fails above 500 ms for the fast path | G2 (fast path) first, G3 (cache) if the full path matters |
| R7 | Tool-name change breaks callers: the main session's tool list in this very conversation already exposes `mcp__savvy-progress__progress/step`; harness-core "forca total" instructions and the upstream savvy-flow skill may name them. | `grep -rn "savvy-progress" ~/Github/harness-core ~/.claude` before M7 | Keep old mod enabled one release; or make the new mod's tool descriptions state the new names and update the callers |
| R8 | Hot reload / `register` re-run loses module state: old mod keeps `ds`, `dsKeys`, `finishedSeenAt` in module variables. | Test: reload mid-session, rows still expire; expiry state in an atom | Atoms only |
| R9 | 1 s redraw and `$.clock.every` cost with long sessions; pane tree size with 200 rows. | Cap roster at 200 (as today), visible rows capped; run a 100-agent test | Slow tick to 2 s past N rows |
| R10 | Provenance / "original code" claim. | Section 5.4 grep sensor in the review of M2..M5 | Reject the PR |
| R11 | Version gate: mod needs the binary with G1; an older binary silently draws less. | `downshift doctor` check added in M7; mod shows `—` for the missing field and README says the minimum version | Doctor fix text |
| R12 | No CI job runs mod suites today (spike 2026-10-23). A new mod without CI regresses silently. | Check whether the `claude` CLI is available to GitHub Actions | Add `scripts/` check plus a documented manual gate; CI job is optional task M9 |
| R13 | Q1/Q2 ambiguity (10 vs 30 s, "no buttons"). | Ask the owner (section 0) | Constants / one-line toggles |

---

## 7. Task breakdown (one PR each, Fibonacci, <= 5)

Estimate basis: 1 = text/config one file; 2 = small change + test; 3 = several files of one domain; 5 = a vertical slice with tests.
Total 40 points + 6 optional. Files per task are exclusive within a wave.

| ID | Task | Pts | Files (exclusive) | Depends on |
|---|---|--:|---|---|
| G1a | `session --panel`: `spawns[].saved_total`, `saved_short`, top-level `summary`; table test | 2 | `internal/telemetry/panel.go`, `panel_test.go` (or new `panel_summary.go` to stay < 300) | none |
| G1b | `accounting.Display.TokensSavedShort`, `Scope` label key in the report block; update `docs/reference/spawn-state.md` contract table | 2 | `internal/accounting/compute.go`, `compute_test.go`, `docs/reference/spawn-state.md` | none |
| G2 | `session --panel --no-report` and `--brief`; Go benchmark with a generated 100 MB log, fast path < 100 ms | 3 | `cmd/downshift/session.go`, `cmd/downshift/sessionpanel.go`, their tests | none |
| M0 | Scaffold `mods/downshift-progress`: plugin.json, hooks.json, router `register.tsx`, `types/index.d.ts`, tsconfig, `wc -l` sensor script, one smoke test; R4 spike result recorded in the PR | 2 | `mods/downshift-progress/{.claude-plugin,hooks/hooks.json,hooks/register.tsx,types,tsconfig.json}`, `scripts/check-mod-size.sh` | none |
| M1 | `shared/downshift` client, parser, key (golden vector, also added to a Go test) | 3 | `hooks/shared/downshift/*`, one new Go test `internal/telemetry/toolusekey_golden_test.go` | M0 |
| M1b | `shared/format`, `shared/i18n` (en, pt-BR), `shared/theme` | 2 | `hooks/shared/{format,i18n,theme}/*` | M0 |
| M2 | `features/roster` (spawn, steps, expiry, reconcile, ticker) + `features/downshift-data` (cadence, single flight) | 5 | `hooks/features/{roster,downshift-data}/*` | M0, M1 (types only) |
| M3 | `features/flow-progress`: `progress`/`step` tools, bounds, opt-in step brief, caller tests | 3 | `hooks/features/flow-progress/*` | M0 |
| M4 | `features/band` (flow row, live-subagent fallback, summary line, auto-hide) | 5 | `hooks/features/band/*` | M1, M1b, M2, M3, G1a (for the live line; tolerant if absent) |
| M5 | `features/agents-pane` + `/ds-agents` command + auto-open option | 5 | `hooks/features/agents-pane/*` | M1, M1b, M2, M3, G1a, G1b |
| M6 | Parity packaging: marketplace entry, mod README (install, options, support matrix, minimum binary), `mods/README.md`, harness-matrix note | 2 | `.claude-plugin/marketplace.json`, `mods/downshift-progress/README.md`, `mods/README.md`, `docs/reference/harness-matrix.md` | M0 (merge after M4 and M5) |
| E1 | Live parity evidence with the owner: terminal and desktop, R1/R2/R3 answers, side-by-side screenshots/notes | 3 | `docs/evidence/downshift-progress-parity-<date>.md` | M4, M5, M6; needs the owner |
| M7 | Cutover: `setup` Mods list and uninstall note, `doctor` minimum-binary check, tests, docs and `llms.txt`, drop savvy entry from the marketplace, register `agents-info` alias, CHANGELOG | 3 | `cmd/downshift/setup*.go`, `cmd/downshift/doctor*.go`, `.claude-plugin/marketplace.json`, `llms.txt`, `docs/guides/{install,claude-code}.md`, `mods/README.md`, `CHANGELOG.md` | E1 |
| M8 | Retire savvy-progress: delete `mods/savvy-progress/`, NOTICE, AGENTS.md list, CI-classify script, historical doc references | 2 | `mods/savvy-progress/**`, `NOTICE`, `AGENTS.md`, `scripts/test-ci-classify-changes.sh`, docs | M7 + one release |
| G3 (opt) | Stat-validated report cache | 3 | `internal/accounting/cache.go`, `cmd/downshift/sessionpanel.go` | G2 (same file) |
| M9 (opt) | CI job running `claude plugin test` for `mods/*` | 3 | `.github/workflows/ci.yml` | R12 answer |

Waves (cap 5 agents including the root; no two parallel tasks share a file):

| Wave | Parallel | Notes |
|---|---|---|
| 1 | M0, G1a, G1b, G2 | disjoint packages; M0 answers R4, which fixes the folder layout |
| 2 | M1, M1b, M2, M3 (after M0 merges) | disjoint `hooks/shared/*` and `hooks/features/*` dirs; M2 needs only M1's types, so M1 merges first |
| 3 | M4, M5, M6 | band, pane, packaging touch different files; M6 merges last |
| 4 | E1 (owner), then M7, then M8 after one release | serial by nature: evidence, cutover, retirement |

Critical path: M0 -> M1 -> M2 -> M4/M5 -> M6 -> E1 -> M7 -> M8. Waits (CI, review, owner evidence) are used by G3/M9 and
the Go tasks, which have no mod dependency because the mod tolerates a missing field by drawing nothing for it.

Plan estimate for the closing comparison: 40 points core, 6 optional, 4 waves.
