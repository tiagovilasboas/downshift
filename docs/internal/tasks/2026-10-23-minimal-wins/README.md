<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# Minimal changes with real token yield (2026-10-23)

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

Owner's question: which small changes are we missing that would raise real token savings in the
Compactor and in model routing, for every harness? Measured on the owner's exec and event logs
(aggregates only) and on the compressor and policy code of `origin/main`. Every task below lives
in a shared layer, so it serves every harness with a shell or spawn channel.

## 1. Where the uncompacted bytes are

Exec log, runs of 2 KB or more, grouped by family, exit and whether they compacted:

| Family | Exit | Compacted | Runs | Raw KB | Why it stayed raw |
|---|---|---|--:|--:|---|
| gh-run | 0 | no | 9 | 2,307 | no `gh run view --log` compressor (V9) |
| go-test | 0 | **no** | 16 | **986** | clean runs the compressor refuses: see 1.1 |
| go-test | ≠0 | no | 30 | 651 | failures pass through (C5, PR #491) |
| go-test | 0 | yes | 17 | 77 | the only compaction so far |
| pnpm-build | 0 | no | 10 | 51 | no compressor |
| gh-pr, git-show, git-log, gh-api | 0 | no | 23 | 99 | readouts, no compressor |

So far the Compactor saved 58 KB of go test output in total; **986 KB of clean go test output
passed through untouched**, 17 times more.

### 1.1 Why a clean `go test` stays raw (probed against `CompressExit`)

| Shape | Result today | Reason code |
|---|---|---|
| `-v` run where a passed test logged (`t.Log`, `t.Logf`) | whole output raw | `go_test_verbose_test_output_preserved` |
| any stray line outside the `ok` lines (a `fmt.Println` in `init`, a deprecation notice) | whole output raw | `go_test_preserved_non_pass_lines` |
| `-cover` with one package | raw | `go_test_too_few_packages_to_compress` |
| `-json` | raw | `unrecognized_format` |
| 2 `ok` lines, `(cached)` lines | raw | `no_compression_gain` (correct) |

A single logged line or a single stray print makes the compressor give up on the whole run. That
is the shape of a large test suite: some test always logs. The fix is to fold, not to give up.

### 1.2 Commands never rewritten

- `go test ./... 2>&1` is refused (`unsafe_character`) while `go test ./... 2>&1 | tail -20` is
  rewritten (`allowlisted_compound`). The bare `2>&1` is the most common agent idiom and only
  merges two streams the wrapper already captures.
- `downshift compact -- "go test ./..."` (one quoted word) is refused: the family check sees one
  argument.
- 108 direct calls were refused with exit 126 (`family other`). Each costs the agent a failed tool
  call plus a plain rerun. **Passing them through is not safe:** the wrapper would become a
  generic runner and a `deny` rule written for the original command would no longer match. The
  fix is on the agent side: a steering text that lists the exact allowlisted families, and a
  refusal message that names them and says to rerun the command plainly.

### 1.3 What the model actually sees

Claude Code truncates Bash output at about 30,000 characters (community reports and the
`BASH_MAX_OUTPUT_LENGTH` variable; not in the official docs, so unverified). For the 18 runs above
30 KB, the bytes beyond the cap never reached the model, so `saved_bytes` overstates tokens
saved there; the real gain of compacting them is that the agent does not rerun the command with
`grep`/`tail` to see the end. The accounting should cap a run's saved bytes at the harness's
visible limit, as data in `HarnessCapabilities`.

## 2. Model routing: the signal we ignore

Claude Code decisions in the log, by agent type: `general-purpose` 73, `Explore` 7, untyped the
rest. `core.Route` receives no agent type at all (`grep AgentType internal/core` is empty); the
adapter reads `subagent_type` only to pin built-ins and to label telemetry. Yet the type is the
strongest cheap signal there is: Claude Code's `Explore` is read-only search, `Plan` is design,
`statusline-setup` is config; Cursor and Codex send `subagent_type` / `agent_type` too. In the
log, 4 of the 7 `Explore` spawns ended `UNKNOWN`/held. Requested models: `claude-opus-5-5` on 42
rows is where the money is; 46 `CLASSIFIER_UNCONFIDENT` and 24 `NO_SIGNAL` on `general-purpose`
are the rows routing leaves alone (C1, PR #492, covers the lookup shapes).

## 3. Tasks (one PR each, Fibonacci points, max 5)

| # | Task | Pts | Layer | Serves |
|---|------|--:|-------|--------|
| W1 | Clean `go test`: fold passed tests that logged (drop their `=== RUN`/log/`--- PASS` block, keep a count) and keep stray non-test lines while still folding the `ok` lines; `-cover` with one package folds too | 3 | `internal/compressor` | all |
| W2 | Rewrite policy accepts a trailing `2>&1` (and `2>&1` before `\| tail -N`); one quoted word with spaces and no metacharacters is split into argv | 2 | `internal/execwrap` policy | all |
| W3 | Agent type as a routing signal: the adapter exposes it through an optional method, core takes it as `Input.AgentType`, and a per-harness map in `HarnessCapabilities` (data, not branches) gives a tier hint (`Explore` → small, `Plan` → mid); the hint is a signal the classifier fuses, never a forced tier, and R-rules still apply | 5 | `core`, caps, adapters | all |
| W4 | Refusal message names the allowlisted families and says "run the command plainly"; the KiroCrew steering text lists them; `setup` installs the same steering text for every harness it finds (Cursor rules dir, Codex `AGENTS.md` note, OpenCode, Antigravity) | 3 | `execwrap`, `setup_steering.go` | harnesses without a rewrite channel |
| W5 | Accounting: cap a run's `saved_bytes` at the harness's visible output limit, stored in `HarnessCapabilities` (Claude Code value to be confirmed in its docs) | 2 | `accounting`, caps | all |
| W6 | `pnpm build` / `npm run build` / `next build` compressor: keep warnings and errors, fold the asset table | 3 | `internal/compressor` | all |
| W7 | Readout compressors for `gh pr view`/`gh pr list` and `git log`: fold to the fields the agent asked for only when the output is a recognised table | 3 | `internal/compressor` | all |

Decisions: W5 needs the Claude Code limit confirmed (owner or docs); W3 changes routing and goes
without auto-merge for the owner's review.

## 4. Order

W1, W2, W3, W4 now in parallel (compressor, policy, core, steering: no shared files). W5 after
the limit is confirmed. W6 and W7 after W1 (same package, different files).
