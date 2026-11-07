# Hook coexistence: two PreToolUse hooks that rewrite one call (2026-11-06)

Downshift by Tiago Vilas Boas
https://github.com/tiagovilasboas/downshift

Question: what happens when Downshift and another `PreToolUse` hook both answer
with an `updatedInput` for the same Claude Code call? Two real tools were used as
the other hook: RTK on `Bash` and cheap-subagents on `Agent`. The guide that
reports the result is [companions.md](../guides/companions.md#hook-coexistence-rtk-and-cheap-subagents-next-to-downshift-measured-2026-11-06).

## What is real and what is a stand-in

| Piece | Status |
|---|---|
| Claude Code | **Real**, `claude --version` = `2.1.319`. One `claude -p` process per scenario |
| Downshift | **Real**, built from this working tree (base `b4acf99`, plus the doctor change in this commit). `downshift claude-code` is the hook command `setup` writes |
| cheap-subagents | **Real**, `cheap_subagents.py` 0.1.4 unmodified, run as the hook. Read in full before running: standard library only, no network call, a local JSONL log (redirected to the run directory). Its settings entry has the shape its own `--install` wrote |
| RTK hook script | **Real**, the published `hooks/claude/rtk-rewrite.sh` (header `rtk-hook-version: 4`) unmodified |
| `rtk` binary | **Stand-in.** Not installed, not run. `bin/rtk` implements only what that script calls (`--version`, `rewrite`) and a pass-through runner. Its rewrite rule is "prefix `rtk ` for go/git/cargo". It proves nothing about the real RTK |
| `go` | **Stand-in** that logs its argv and its parent process's command line, prints a failing test and exits 3 |
| Anthropic API | **Mock**, `mockapi.py`, bound to 127.0.0.1. It returns one scripted `tool_use` and then `end_turn` |

Nothing left the machine. Each run uses `env -i` with only `PATH`, a throwaway
`HOME`, `ANTHROPIC_BASE_URL` pointing at the local mock, a dummy key, and the
telemetry and update switches off; no proxy variable is passed. The two
third-party files were downloaded once, hash-checked (`fetch_hooks.sh`) and never
executed with real data.

Pinned inputs (upstream commit ids could not be read from the evidence host, so
the hash of the bytes that ran is the pin):

| File | sha256 |
|---|---|
| `cheap_subagents.py` (main, 2026-11-06) | `9403ce224eb7057cb7f7b7c41eaa59ee0e2f468a143c388a6e86bba14f30f35a` |
| `rtk-rewrite.sh` (`hooks/claude/`, master, 2026-11-06) | `d4cd1e04cbdacd52d24eefb1e1500ed76008b9710ed4e9a1de0bf36b1893267d` |
| Claude Code hooks reference (`/docs/en/hooks.md`) | `55757f0fbe69bc20bb184c53c41f9d606e15bb4f8c3cd2a9b088c06b4357b906` |
| Claude Code hooks guide (`/docs/en/hooks-guide.md`) | `a137a70720739938d89001a688a3e33f4516374dac8a1c2afbece1df61d37fb3` |

## What the documentation says

Quoted in [companions.md](../guides/companions.md#what-claude-code-documents).
The one rule that settles the question is in the guide's limitations list, not in
the hooks reference: *"When multiple `PreToolUse` hooks return `updatedInput` to
rewrite a tool's input, Claude Code doesn't merge the rewrites or guarantee which
rewrite the tool runs with. Avoid having more than one hook rewrite the same
tool's input."* The reference page is silent on this; a first read of the
reference alone would have concluded "undocumented".

## Method

[`run_matrix.sh`](hook-coexistence-2026-11-06/run_matrix.sh) runs 38 scenarios,
each in a fresh directory ([`run_scenario.sh`](hook-coexistence-2026-11-06/run_scenario.sh)
refuses to reuse one):

1. build `settings.json` for the hooks and order under test
   ([`mksettings.py`](hook-coexistence-2026-11-06/mksettings.py));
2. start the mock API; run `claude -p` against it, once;
3. read what actually happened: which wrapper the fake `go` was started by
   (its parent process), the exit code in the tool result, the model on the
   subagent's own API request, whether the `[model:…]` tag survived in the
   prompt, and the order in which Claude Code's debug log says the hooks
   returned ([`summarize.py`](hook-coexistence-2026-11-06/summarize.py));
4. run `downshift doctor --json --harness claude-code` on that same `HOME`.

Timing probe: `delay-wrap.sh` sleeps before running one hook unchanged
(`DELAY=rtk=0.5`), to separate "later in the file", "faster" and "last to return".
`NO_ALLOW=1` removes the `Bash`/`Agent` allow rule so a hook's own `allow` has to
supply the permission.

## Results

Full table: [`results.tsv`](hook-coexistence-2026-11-06/results.tsv). Claude Code's
debug lines: [`claude-debug-excerpts.txt`](hook-coexistence-2026-11-06/claude-debug-excerpts.txt).

### Downshift + RTK on `Bash` (`go test ./...`)

| Scenario | Applied | Double wrap | Exit code | Hooks returned in order |
|---|---|---|---|---|
| Downshift only | `downshift compact -- go test ./...` | no | 3 | downshift |
| RTK only | `rtk go test ./...` | no | 3 | rtk |
| Both, either file order, natural timing (12 runs: 2 + 5 + 5) | `downshift compact -- …` in 12 of 12 | no | 3 | rtk, then downshift |
| Both, RTK delayed 0.5 s (either file order) | `rtk go test ./...` | no | 3 | downshift, then rtk |
| Both, Downshift delayed 0.5 s (either file order) | `downshift compact -- …` | no | 3 | rtk, then downshift |

Across the 18 runs with both hooks: **0 double-wrapped**, and the hook that
returned last supplied the applied input in **18 of 18**. The Claude Code log lists
`modified tool input keys: [command, description]` for both hooks in every one of
them, so the loser's rewrite was accepted and then replaced, not rejected.

Permission, no allow rule, headless `-p` (nothing can answer a prompt):

| Hooks | Outcome |
|---|---|
| Downshift only | denied: `This command requires approval` (Downshift emits no `permissionDecision`) |
| RTK only | ran (`permissionDecision: allow`, reason `RTK auto-rewrite`) |
| Both, either order | **ran Downshift's rewritten command**; the only `allow` came from RTK, whose rewrite was discarded |

### Downshift + cheap-subagents on `Agent`

The subagent's own API request carries the model that was applied. Requested
model `opus` in every prompt.

| Prompt | Downshift alone | cheap-subagents alone | Both (late hook decides) |
|---|---|---|---|
| `implement pagination for the users endpoint` | `opus` (holds, answers `{}`) | `sonnet` | `sonnet`, no conflict: Downshift had nothing to rewrite |
| `[model:sonnet] rename the userId variable to userIdentifier` | `haiku`, tag left in the prompt | `sonnet`, tag stripped | Downshift last: `haiku`, tag left. cheap-subagents delayed so it returns last: `sonnet`, tag stripped |
| `rename the lock helper in the concurrency module` | `haiku` | model unset, so the child inherits the parent (`sonnet` here) | Downshift last: `haiku`. cheap-subagents delayed: `sonnet` (inherit) |

In all 8 Agent runs where the two answers differed, the hook that returned last
decided. With natural timing Downshift returned last.

What Downshift recorded when it lost
([`lost-rewrite-record.txt`](hook-coexistence-2026-11-06/lost-rewrite-record.txt),
run `agent2-downshift_cheap-cheap-slow`): the subagent was served
`claude-sonnet-5-5`; `events.jsonl` says `final_model claude-haiku-5-5`,
`outcome rewrite_emitted`; `downshift stats` prints `Models: haiku 1`. The
fixture wired only `PreToolUse`, so no `SubagentStop` hook was there to compare
the served model with the decision.

### Checks without a live run

Run through the real `downshift claude-code` hook on a `Bash` payload:

| Command given to the hook | Answer |
|---|---|
| `go test ./...` | rewrite to `downshift compact --session=… -- go test ./...` |
| `rtk go test ./...` | `{}` (left alone) |
| `downshift compact -- go test ./...` | `{}` (left alone) |
| `go test ./...` with `DOWNSHIFT_EXEC=0` | `{}` (the documented switch turns the rewrite off) |

### `downshift doctor`

[`doctor-output.txt`](hook-coexistence-2026-11-06/doctor-output.txt), from the
settings files of the runs above, with the doctor built from this change:

- Downshift alone, RTK alone: no `hook overlap` row.
- Downshift + RTK: one warning, `RTK also rewrites the command; one rewrite is
  lost`, with a `fix` (turn Downshift's rewrite off with `DOWNSHIFT_EXEC=0` /
  `"exec": false`, or remove RTK's entry).
- Downshift + cheap-subagents: one warning, `cheap-subagents also rewrites the
  model; one rewrite is lost`, with a `fix`.
- The delayed runs show no row: the timing probe launches the hook through
  `delay-wrap.sh`, so the config no longer names the tool. That is a property of
  the probe, not of the check.

Before this change doctor reported neither pair.

## Limits of this evidence

- One Claude Code version (2.1.319), one machine, one fixture command. Cursor,
  Codex, Antigravity and KiroCrew were not measured.
- The RTK result is for the published hook script plus a stub binary. The real
  `rtk` rewrite rules, its deny/ask handling, `rtk hook claude`, and the exit
  code of a real `rtk <command>` are unmeasured. "Exit code 3 survives" is shown
  for `downshift compact` (real) and for the stub (pass-through) only.
- "Last to return wins" held in all runs but is not a documented rule; the
  documentation says the winner is not guaranteed.
- A run with all of Downshift's hooks wired next to cheap-subagents ended with a
  different Downshift decision for the same prompt, so it did not exercise the
  lost-rewrite case. Whether the `SubagentStop` honor check flags a lost model
  rewrite is not shown here.
- Ask and deny rules together with RTK, and hooks in project settings or in
  plugins, were not run. Doctor does not read them either.
- Method notes: a first attempt at the delayed cheap-subagents entry broke its
  exec form (`command` + `args`) and spawned nothing; those runs were discarded
  and redone with the fixed builder. An early summarizer matched the word
  `downshift` in a file path and reported a double wrap for the RTK-only run; the
  raw logs were correct and every row in `results.tsv` was recomputed with the
  fixed summarizer.

## Reproduce

```bash
go build -o "$FX_ROOT/bin/downshift" ./cmd/downshift
docs/evidence/hook-coexistence-2026-11-06/fetch_hooks.sh "$FX_ROOT/hooks"
FX_ROOT=… DOWNSHIFT_BIN="$FX_ROOT/bin/downshift" docs/evidence/hook-coexistence-2026-11-06/run_matrix.sh
```

Needs `claude`, `node`, `python3`, `jq` and `ps` on the machine; `FX_ROOT` is an
empty directory the runs may write to. Run it from outside the repository: a
Downshift command run from inside it can write project state there.
