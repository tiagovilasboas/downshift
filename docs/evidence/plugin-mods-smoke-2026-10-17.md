<!-- Copyright (c) 2026 Tiago de Carvalho Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# Plugin and mods smoke test, headless Claude Code 2.1.302 (2026-10-17)

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

What this page answers: do the Downshift plugin and the sibling mods load under `claude -p`, and does routing still work
when they are loaded? It is a smoke test: three sessions, one per round, one operator. Aggregates only; no prompts or paths
are published.

## Setup

- Claude Code 2.1.302, `downshift` on PATH (dev build), real account, model `sonnet` (resolved to `claude-sonnet-5-5`).
- Plugins loaded with `--plugin-dir` only, never installed at user scope: the repository root is the Downshift plugin; the
  mods are `mods/savvy-progress`, `mods/filetree` and `mods/downshift-router`.
- `--setting-sources project,local`. The root plugin's `hooks/hooks.json` carries the same four hooks that `downshift setup`
  writes into the user's settings, and Claude Code runs both separately, so with user settings loaded every spawn would be
  routed twice. Excluding user settings leaves the plugin hooks as the only routing path. This is deliberate and is the
  setup for rounds 1 and 2.
- A scratch working directory holding copies of the 99 Go files of `internal/core`, a fresh `DOWNSHIFT_EVENT_LOG` per round, `--debug-file` per round.
- Flags: `--output-format stream-json --verbose --max-budget-usd 0.6 --allowedTools "Agent" "Read" "Bash(ls*)"`.
- Prompt (on stdin): use exactly three Agent subagents, one call each (list the files; count the `.go` files with `ls`; read
  the first line of one file), each replying in one line, then a one-sentence summary.
- Round 3 sets `DOWNSHIFT_MOD_ROUTER=1` and `DOWNSHIFT_MOD_ROUTER_LOG=<file>`.

`claude plugin validate`, last line of each:

| Directory | Verdict |
|---|---|
| `.` (Downshift plugin) | `Validation passed with warnings` (a note on `cache-tax` reading `NO_COLOR`) |
| `mods/savvy-progress` | `Validation passed` |
| `mods/filetree` | `Validation passed` |
| `mods/downshift-router` | `Validation passed` |

## Rounds

| Round | Plugins | Finished | `total_cost_usd` | `modelUsage` (tokens in / out) | Decisions in the event log |
|---|---|---|--:|---|---|
| 1 | Downshift | yes, `success`, 4 turns | $0.230 | Sonnet 5.5 18 / 770; Haiku 5.5 1,244 / 23 | 3 `allow`: one `unknown` baseline held (TRIVIAL, small), two Sonnet kept (MEDIUM, mid). 0 rewrites |
| 2 | Downshift + savvy-progress + filetree | yes, `success`, 4 turns | $0.173 | Sonnet 5.5 14 / 592; Haiku 5.5 4 / 139 | 1 `rewrite_emitted` Sonnet to Haiku (TRIVIAL, `DOWNSHIFT`), 2 `allow` Sonnet kept (MEDIUM). Reason code on all three: `no_advisory_signal` |
| 3 | Downshift + downshift-router (`DOWNSHIFT_MOD_ROUTER=1`) | yes, `success`, 4 turns | $0.046 | Sonnet 5.5 14 / 539; Haiku 5.5 4 / 137 | Hook path: 1 `rewrite_emitted` to Haiku, 2 `allow`. Mod path: 1 `allow` on the already-Haiku spawn, 1 `allow` with unknown model, 2 `baseline` lines (control group), see below |

`total_cost_usd` is the figure of the last `result` line of the stream (one session printed four `result` lines, the cost is
cumulative). The three costs are not comparable with each other: the prompt cache was cold in round 1 (158k cache-read
tokens, 74k cache-creation) and warm in rounds 2 and 3, and nothing here is a cost claim.

**Round 1, plugin alone.** The plugin loaded (`downshift@inline` in the init event, `Read hooks.json for plugin downshift`
in the debug log) and its `PreToolUse:Agent` hook ran for all three spawns. The first spawn had no known parent model
(`requested_model` `unknown`), so it was held, not rewritten: the known first-spawn transcript race documented in
[subagent-routing-ab-2026-10-15.md](subagent-routing-ab-2026-10-15.md). Each decision has its `resolved`, `child_ended` and
`usage` follow-ups in the log (3 of each). No hook failure.

**Round 2, plugin and two mods.** `savvy-progress` and `filetree` loaded as hooks modules (`hooks module savvy-progress@inline
loaded`, `filetree@inline loaded`, `/filetree` command registered, `ui_panes` event in the stream). No error line from either.
The first spawn was rewritten to Haiku; its `usage` record shows `final_model` `claude-haiku-5-5`, and the session's
`modelUsage` carries a Haiku entry of 4 input, 139 output and about 21k cache-read tokens that round 1 does not have.

**Round 3, plugin and the router mod.** Settings hooks were excluded and the plugin hook was present, so both paths were live.
Observed: both. The plugin's `PreToolUse:Agent` hook ran first (45 ms) and wrote `model: haiku` on the first spawn; then the
mod's `agent.spawn` hook ran the same binary (`downshift exited 0 in 33 ms`). The mod's trace log has three lines, none
writing a model: the first saw `requested_model` `haiku` (already rewritten by the hook), so its decision was an
`allow` Haiku to Haiku; the second and third saw no model; the third was drawn into the control group
(`DOWNSHIFT_CONTROL_SHARE`), spawn unchanged. So the event log holds twice the decision lines (6 against 3) of rounds 1 and 2
(3 hook decisions plus 3 more `allow` / `baseline` lines from the mod's runs of the binary), which is the double-routing shape described in
[mod-router.md](../design/mod-router.md#6-risks). Here the second path did not change the outcome, because it wrote nothing,
but the telemetry double-count is real.

## What it shows

- The Downshift plugin loads under `claude -p --plugin-dir` and its hooks run on every `Agent` spawn.
- Routing decisions are emitted and recorded: a `rewrite_emitted` Sonnet to Haiku in rounds 2 and 3, decisions with reason
  code `no_advisory_signal`, and `usage` records linked to them.
- `savvy-progress`, `filetree` and `downshift-router` load as hooks modules next to the plugin, and none of them stopped
  routing, failed a session or produced an error line.
- In rounds 2 and 3 a Haiku entry with a child-sized token count appears in `modelUsage`. It is consistent with the rewrite
  being honored, but see the limits below.

## What it does not show

- Nothing is drawn under `claude -p`. The panes, bands and `/filetree` listing are not visible here; the evidence is the debug
  log and the `ui_panes` stream event, not a rendering.
- n = 1 per round. No variance, no cost comparison, no quality or saving claim. Cost differences between rounds are cache
  state and agent variation.
- The honored model is not verified. A Haiku `modelUsage` entry is billed to Haiku, but the harness also makes small Haiku
  calls of its own (round 1 has one: 1,244 input, 23 output, no cache), so round 2 and 3's entry is suggestive, not a link
  from one spawn to one billed call.
- Round 1 shows the first-spawn unknown-baseline hold; rewrites are not guaranteed on every run.
- The double routing in round 3 is observed, not fixed; the mod stays opt-in and off by default.
- Other harnesses, user-scope installs and a session with user settings loaded were not run.

## Reproduce

```bash
S=$(mktemp -d); mkdir "$S/cwd"; cp internal/core/*.go "$S/cwd/"; cd "$S/cwd"
for d in . mods/savvy-progress mods/filetree mods/downshift-router; do (cd <repo> && claude plugin validate $d | tail -1); done
echo 'Use exactly three Agent subagents, one call each: (1) list the files in the current directory; (2) count the .go files in the current directory using ls; (3) read the first line of classifier.go. Each must reply in one line. Then summarize in one sentence.' > "$S/prompt.txt"
# round 1; add the two mod dirs for round 2; add mods/downshift-router with the env below for round 3
DOWNSHIFT_EVENT_LOG="$S/r1-events.jsonl" claude -p --setting-sources project,local --plugin-dir <repo> \
  --output-format stream-json --verbose --debug-file "$S/r1-debug.log" --max-budget-usd 0.6 --model sonnet \
  --allowedTools "Agent" "Read" "Bash(ls*)" < "$S/prompt.txt" > "$S/r1-out.jsonl"
# round 3 only: DOWNSHIFT_MOD_ROUTER=1 DOWNSHIFT_MOD_ROUTER_LOG="$S/r3-router.jsonl"
```

Pass the prompt on stdin: `--allowedTools` is variadic and swallows a trailing prompt argument. Read the log with
`python3 -I` (one JSON object per line; `outcome`, `requested_model`, `final_model`, `decision_intelligence.reasons`).
Total spend for the three rounds: $0.45 at list price.

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift
