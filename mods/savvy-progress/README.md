# savvy-progress (vendored mod)

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

This directory is a vendored copy of **savvy-progress**, a Claude Code mod by
johnnyvizz, MIT License ([LICENSE](LICENSE)). It ships in the Downshift
marketplace as its own plugin (`/plugin install savvy-progress@downshift`):
Claude Code loads one hooks module per plugin and refuses a second
`session.start` hook in it, and the mod's state, tool names
(`mcp__<plugin>__…`) and panel are keyed by its plugin name, so it cannot be
folded into the `downshift` plugin unchanged.

| | |
|---|---|
| Upstream | https://github.com/JohnnyVizz/claude-kit, path `plugins/savvy-progress` |
| Version | 1.2.1 upstream; shipped here as **1.2.8** (Downshift build: only `plugin.json` `version` differs, so Claude Code replaces a cached copy of 1.2.1 that still has upstream's purple) |
| Commit | `c871c153ead54ad9547cc717d0f3e5a676c73986` (2026-10-13) |
| Files | `hooks/hooks.json`, `types/index.d.ts` byte-identical to upstream; `.claude-plugin/plugin.json` byte-identical except `version` (1.2.8); `hooks/register.tsx` byte-identical except the Downshift local patch below; plus Downshift's own `hooks/downshift.test.ts`; three of the four `hooks/*.test.ts(x)` files byte-identical (`hooks/cost.test.ts` is gated by `SHOWN_DOLLARS`, see the local patch) (they import nothing by relative path) |
| Loaded by | its own `hooks/hooks.json`, `"modules": ["./register.tsx"]`; listed in [`../../.claude-plugin/marketplace.json`](../../.claude-plugin/marketplace.json) as `savvy-progress`, source `./mods/savvy-progress` |
| Needs | Claude Code 2.1.287 or later (mods on by default); a pane for `/agents-info` (desktop docks it; the terminal needs the fullscreen layout) |

## Downshift local patch

Owner decision 2026-10-19: the look follows the Downshift site (black first,
white and grey carry the interface, green only as a small accent;
`docs/brand/downshift-brand-tokens.json`). The only local change to upstream is
in `hooks/register.tsx`: colours, the shown tier words and one simpler sprite, no
behaviour; plus the `version` line of `.claude-plugin/plugin.json` (1.2.1 to
1.2.8, 2026-10-20; 1.2.2 on 2026-10-19 was superseded before the agents panel changes reached the cache). Claude Code keys its plugin cache by that version, so a
machine that installed 1.2.1 before the colour patch keeps upstream's purple
`#8f8cf4` until the version changes. The panel's header tile, summary lines and per-agent rows show the tokens Downshift saved in the session (`downshift session --panel`, `work_saved` + `cache_saved`) instead of the estimated dollar cost; the dollar estimate is no longer drawn. The reapplyable diff is [`downshift-local.patch`](downshift-local.patch),
regenerated 2026-10-20 against the pinned upstream commit.

| In `hooks/register.tsx` | Upstream | Downshift | Why |
|---|---|---|---|
| `ACCENT` (running bar, dot) | `#8f8cf4` | `#f5f7fa` | white while running |
| `DONE` (finished bar, dot, check) | `#5fbf8f` | `#22c55e` | green means done |
| `PILL_INK` (pill text; was a literal `#ffffff`) | `#ffffff` | `#0a0a0a` | brand black on a white or green pill |
| `TIER_COLOR` | violet, orange, amber, blue, teal | grey ramp by lightness: `fable` `#FFFFFF`, `heavy` `#D6DAE2`, `careful` `#AAB0B8`, `medium` `#868A90`, `light` and `other` `#5C6168` | tier by lightness; green only for done, red for failed |
| Pixel crab | one costume per role (hard hat, star, palette, flag, ...) | one monochrome crab in greys (`CLAY` `#868A90`, claws `#D6DAE2`); one `#22c55e` pixel once the agent is done | no per-role costumes |
| Tier words shown (`tier` strings, all three languages) | `light`, `medium`, `careful`, `heavy`, `fable` | `small`, `medium`, `frontier`, `frontier`, `frontier` | same words as the site and `downshift try`; the internal tier keys and the tool schema are unchanged |

### Second part: what Downshift did (2026-10-19)

Owner decision: the panel and the band show what Downshift did. The mod only
draws; every number and label is computed by Downshift
(`downshift session --panel`, see
[docs/reference/spawn-state.md](../../docs/reference/spawn-state.md)).

| In `hooks/register.tsx` | What it does |
|---|---|
| `dsRefresh` | runs `downshift session --panel` (`$.process.run`, 2 s timeout, at most every 3 s on render; forced at spawn and at turn end). Any failure leaves the panel exactly as upstream draws it |
| `dsKeyOf` | sha256 of `harness-downshift-session-v1:` + the spawn's `tool_use_id` (Web Crypto), so a panel row finds Downshift's record for that spawn |
| row | the model line gains one short status and, when tokens moved to a smaller model, `↓` and their count: `opus → sonnet (mid) · ↓58k`; `held`, `kept`, `control`, `paused`, `blocked` stand alone |
| header and band | one short line: `Downshift · N switched · M held` (zero counts are left out; `Downshift · on` when tokens moved but nothing was switched or held). When `session --panel` reports `effort_changed` above zero, `N switched` becomes `N model · M effort` (`model_switched`, `effort_changed`); an older binary without those fields keeps `N switched`. The first tile reads `Downshifted` (tokens that ran on a smaller model) and a fourth tile, `Compactor`, shows the bytes the Compactor cut (`compactor.saved_bytes` from `session --panel`, only when present), or `nothing to cut` when no output was compacted and the raw output was under 4 KB; the summary suffix reads `Compactor N cmds · X saved`. The two are never added together, because one is measured tokens and the other measured bytes. The pane's grid counts every row placed over it, the Downshift line included, so the pane is never shorter than its content | The work/cache percentages and the per-type line are gone from the mod (the CLI still prints them) |

| finished rows (`shownFinished`, `FINISHED_SHOW_MS`) | a finished agent's row leaves the panel 10 s after the panel first sees it finished; running rows are never hidden. The band's `N done` and the section label still count every finished agent. The band's `×N` badge (the crew button) counts the same rows the panel lists: running agents, finished agents still shown by `shownFinished` at the same render time, and planned tasks. The render time is the live clock, not the stored `now` (`liveNow`) |

| band fallback (`derivedFlow`) | when no flow is registered, or its total is 0, **and Downshift is working in this session** (`downshift session --panel` for the session names at least one decided spawn), the band is drawn from the subagents the mod already tracks: title `Subagents`, `N running · M done`, bar `M/(N+M)` over the current batch (spawns since the oldest running one; a finished batch lingers 30 s). A registered flow with a total keeps upstream drawing |

| dismiss | the `✕` clears the registered flow and records the time; the band that follows the live subagents (the fallback above) then draws only agents started after it, so the button works and the band returns with the next agent. The `✕` sits at the right edge of the row, the count `×N` stays with the bar. |
| session | `dsRefresh` passes `--session=<id>` (`$.session.id()`, else `CLAUDE_CODE_SESSION_ID`). With no id the summary line is not drawn at all, so another session's figure can never appear; row labels stay (keyed by the spawn's own tool call) |

**Behaviour change, owner-requested:** while Downshift is working in the session, the band above the prompt also appears when subagents run with no registered flow (see the band fallback row above). When Downshift is not active, or its data is missing or the session unknown, the mod draws exactly what upstream draws.

Nothing here touches routing and it spends nothing. Downshift's lines carry
tokens only, never dollars (owner decision 2026-10-19): "saved" means the
agent's tokens ran on a cheaper model than the one requested, not that fewer
tokens were used. `downshift report` compares routed and control token counts;
it prints no dollar values. The agents panel's own per-agent cost estimate is
upstream's and is unchanged. The new
test file `hooks/downshift.test.ts` is Downshift's own (not upstream). The
spawn and the Downshift hook must see the same `tool_use_id`; if they do not
(unverified live), rows show no label and the summary still draws.

Re-vendor, then `git apply mods/savvy-progress/downshift-local.patch` (one patch
for both parts; it applies to the pinned upstream commit).

## What it does

- **Progress bar above the prompt**: the flow's title, phase, accepted tasks out
  of planned and a crew button that opens the panel. It appears once something
  reports progress through `mcp__savvy-progress__progress` (the savvy-flow
  skill does) or a `savvy-*` worker starts; ✕ dismisses it.
- **Agents panel** (`/agents-info` toggles, `/agents-info open` brings it
  forward): running, finished and planned subagents with model, effort, task
  steps, context fill, tokens, estimated cost and time. Any subagent is listed,
  not only savvy-flow's; the panel opens by itself for `savvy-*` workers, or
  for any subagent with the option `autoOpen: any`.
- **Tools**: `mcp__savvy-progress__progress` (main session only; a call from a
  subagent is ignored) and `mcp__savvy-progress__step` (subagents report their
  own steps; listed up front, not behind ToolSearch). For `savvy-*` workers
  only, one line asking for step reports is appended to the spawn prompt.

**It spends nothing.** No model request, no network: it draws from the
`agent.spawn`, `turn.step` and `turn.complete` events Claude Code already
raises. The cost column is the mod's own price table (`PRICES` in
`hooks/register.tsx`, list prices of 2026-10-10, cache writes at the 1-hour
rate), not Downshift's catalog; Downshift's `stats` and this panel can differ.

## What it reads, runs, sends and persists

From `claude plugin validate mods/savvy-progress` (2.1.302):

- hooks: `session.start`, `command.run{agents-info}`,
  `tool.call{mcp__savvy-progress__progress}`, `tool.call{mcp__savvy-progress__step}`,
  `tool.call{Agent}`, `agent.spawn`, `turn.step`, `turn.complete`,
  `ui.render{Pane savvy-agents}`, `ui.render{AbovePrompt}`.
- calls: `$.clock.every`, `$.clock.now`, `$.command.register`, `$.env.get`,
  `$.settings.read`, `$.state.get`, `$.state.set`, `$.tool.register`,
  `$.ui.close`, `$.ui.open`, `$.ui.panes`, `$.ui.resolve`.
- env reads: `LANG`, `LC_ALL`, `LC_MESSAGES` (locale for the panel language);
  env writes: none. No `$.http`, `$.fs`, `$.process`, `$.model` or `$.store`.
- Settings: reads Claude Code's `language` key through `$.settings.read`.
- Persists: nothing across sessions. Session state (`$.state`, keys
  `savvy-progress.flow/agents/panel/now`) holds the flow and the subagent rows:
  description, type, model, effort, token counts, cost estimate, times. The
  subagent prompt is read only to append the step line for `savvy-*` workers.

## Options

`language` (`auto`, `en`, `ru`, `he`) and `autoOpen` (`savvy`, `any`), in
`/config` under savvy-progress.

## Both forms installed

If `savvy-progress@claude-kit` (the upstream marketplace install) is also
enabled, two copies draw the bar, register `/agents-info` and both tools.
Uninstall one (`/plugin uninstall savvy-progress@claude-kit`).

## Update procedure

Re-vendor, never edit in place:

```bash
git clone --depth 1 https://github.com/JohnnyVizz/claude-kit.git /tmp/claude-kit
rm -rf mods/savvy-progress && mkdir -p mods/savvy-progress
cp -r /tmp/claude-kit/plugins/savvy-progress/.claude-plugin /tmp/claude-kit/plugins/savvy-progress/hooks \
      /tmp/claude-kit/plugins/savvy-progress/types mods/savvy-progress/
cp /tmp/claude-kit/LICENSE mods/savvy-progress/LICENSE
claude plugin validate mods/savvy-progress && claude plugin test mods/savvy-progress
```

Then restore this README, update the version and commit here and in
[NOTICE](../../NOTICE), and read the upstream diff for new calls
(`claude plugin validate` lists them) before committing.

## Verify

```bash
claude plugin validate mods/savvy-progress   # hooks: ... calls: ... env reads: LANG, LC_ALL, LC_MESSAGES
claude plugin test mods/savvy-progress       # 36 tests across 5 files
```
