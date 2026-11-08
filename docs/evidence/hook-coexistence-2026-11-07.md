<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# Hook coexistence: Downshift next to other input-rewriting hooks, 2026-11-07

Downshift by Tiago Vilas Boas
https://github.com/tiagovilasboas/downshift-ai

Question: what happens when two PreToolUse hooks rewrite the same tool input on Claude Code? Downshift
rewrites an allowlisted `Bash` command and the model of an `Agent` call. The runs paired it with another
project's hook that rewrites `Bash` commands, and with cheap-subagents, which rewrites the model of an
`Agent` call. This is a measured answer, not a reading of the docs: the Claude Code hooks page says matching
hooks "run in parallel" and gives a precedence for conflicting `permissionDecision` values (`deny` over
`defer` over `ask` over `allow`), but it does not say how two `updatedInput` results combine.

## Result

**When two hooks rewrite the same input, only one rewrite is applied, and it is the hook that finishes
last. The order of the entries in the settings file does not matter.** Nothing warns either hook. There is
no double wrapping, because each hook rewrites the original input in parallel and the harness keeps one
answer whole.

| Pair | Tool | Runs | Applied, natural timing | Applied, other hook delayed 0.4 s |
|---|---|--:|---|---|
| Downshift + cheap-subagents | Agent | 8 (4 per file order) | Downshift 8 of 8 (haiku) | cheap-subagents 6 of 6 (sonnet) |
| Downshift + a shell-command rewriter | Bash | 8 (4 per file order) | Downshift 6 of 8, the other hook 2 of 8 | the other hook 4 of 4 when it was not delayed, Downshift 2 of 2 when the other hook was delayed |

Each hook alone behaved as designed (Agent: Downshift haiku, cheap-subagents sonnet; Bash: Downshift
`downshift compact --session=... -- go test ...`, the other hook its own rewrite; no hooks: the original
input).

Natural timing decides the Agent pair because Downshift's hook took 65 to 123 ms and cheap-subagents 21 to
63 ms, so Downshift finished last in every run. On the Bash pair the two hooks took 20 to 57 ms each and
overlapped, so the winner changed between runs (the other hook won twice, once per file order).

## Method

- Claude Code 2.1.319, `claude -p` with `--settings <file>` and `--setting-sources project`, so nothing was
  read from or written to `~/.claude`. Hooks ran from a scratch project (one Go package, two passing tests).
- Downshift built from source, state dir isolated with `DOWNSHIFT_STATE_DIR`. The other `Bash` hook was
  another project's hook, built from its source and registered only in the scratch settings file. cheap-subagents commit
  `3f857464` (2026-10-23), loaded from a settings file, never installed.
- Every hook ran through a logging wrapper that recorded its stdin, stdout, start time and duration and
  passed both through unchanged. The PostToolUse payload carries the input the harness actually applied
  (`model` for Agent, `command` for Bash); that is the observation channel for "which rewrite won".
  The model that ran was confirmed from `modelUsage` in the stream-json result.
- Agent scenario: session on `opus`, one `Agent` call with `subagent_type` general-purpose and a prompt
  starting `[tier=small]` (the hint Downshift reads from the start of the prompt). Downshift answers
  `haiku`; cheap-subagents answers `sonnet`; they differ, so the winner is visible.
- Bash scenario: session on `haiku`, one `Bash` call with `go test -count=1 ./...`.
- Delay runs add a fixed sleep after one hook finishes (`COEXIST_DELAY`), to flip which hook is last.
- 33 retained live runs, about USD 1.01 in total.

## What the runs also showed

- Downshift reads its tier hint from the start of the spawn **prompt**, not the description. A first set of
  runs put `[tier=small]` in the description and Downshift correctly kept the model; those runs were
  discarded and repeated.
- cheap-subagents answers `permissionDecision: "allow"` on every Agent call it rewrites. Downshift sets no
  permission decision on a rewrite.

## Not verified

- **Failing commands.** Every Bash run passed, so exit-code preservation under each pair was not tested here.
- **Other harnesses.** Cursor, Codex, Antigravity and KiroCrew were not run. The doctor check is computed
  for every harness that can rewrite and reads its matchers from the setup target table, but only Claude Code
  was observed.
- **Where the other hook lives.** Doctor reads the user hook file. A hook registered in a project
  `.claude/settings.json`, in a plugin, or in managed policy is not read.
- **Stability.** The winner rule is observed behavior of one Claude Code build. The docs do not promise it.
- **Load.** Timing was measured on one idle machine, three to four runs per cell.

Guide: [companions.md](../guides/companions.md#hook-coexistence-two-hooks-that-rewrite-the-same-input).
