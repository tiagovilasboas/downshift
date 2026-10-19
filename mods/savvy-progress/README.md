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
| Version | 1.2.1 |
| Commit | `c871c153ead54ad9547cc717d0f3e5a676c73986` (2026-10-13) |
| Files | `.claude-plugin/plugin.json`, `hooks/hooks.json`, `types/index.d.ts` byte-identical to upstream; `hooks/register.tsx` byte-identical except the Downshift colour patch below; the four `hooks/*.test.ts(x)` files byte-identical (they import nothing by relative path) |
| Loaded by | its own `hooks/hooks.json`, `"modules": ["./register.tsx"]`; listed in [`../../.claude-plugin/marketplace.json`](../../.claude-plugin/marketplace.json) as `savvy-progress`, source `./mods/savvy-progress` |
| Needs | Claude Code 2.1.287 or later (mods on by default); a pane for `/agents-info` (desktop docks it; the terminal needs the fullscreen layout) |

## Downshift colour patch

The only local change to upstream: the bar uses the Downshift brand green
(`docs/brand/downshift-brand-tokens.json`, accent `#22C55E`) instead of the
upstream violet, and the percentage pill uses the brand ink `#0D1117` instead
of white, because white on green has poor contrast.

| Constant in `hooks/register.tsx` | Upstream | Downshift |
|---|---|---|
| `ACCENT` (running) | `#8f8cf4` | `#22c55e` |
| `DONE` (finished) | `#5fbf8f` | `#16a34a` |
| `PILL_INK` (pill text; was a literal `#ffffff`) | `#ffffff` | `#0d1117` |

Tier colours in the agents panel are untouched: they tell tiers apart. When
re-vendoring a newer upstream, reapply these three lines and rerun
`claude plugin test mods/savvy-progress`.

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
claude plugin test mods/savvy-progress       # 24 tests across 4 files
```
