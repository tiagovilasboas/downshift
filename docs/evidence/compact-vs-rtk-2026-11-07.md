<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# Compaction against RTK on the same commands (2026-11-07)

Downshift by Tiago Vilas Boas
https://github.com/tiagovilasboas/downshift

[bench.md](../reference/bench.md) says its old rtk column is not like for like and tells the reader to run
the real command under both tools. This page does that, once, on one machine. It is rung B evidence
([ladder](../guides/evidence.md)): bytes on small synthetic projects, not tokens, not money, and not agent
outcomes.

## Verdict

- **Bytes, all 15 runs:** RTK cuts 39.3%, Downshift cuts 63.0% with CCR on (the default) and 36.0% with CCR off.
  One command, `git log -n 30`, is 36% of the raw bytes, so read the table by class, not by the total.
- **Failing runs:** together, RTK saves 10.4% of the bytes and Downshift 6.2% to 7.1%. Downshift leaves Go, pytest and tsc failures raw by design
  (non-zero exit) and keeps every fact. RTK cuts Go and pytest failures by about half and, in the Go run, drops the
  two stack frames in the user's own code (`p.go:3`, `p_test.go:6`), which `rtk recall` brings back. RTK's output
  for `tsc` and `vitest` failures was larger than the raw output (392 against 311 bytes, 3349 against 1857).
- **Passing runs:** RTK is much terser (119 bytes against 448 for Downshift, 572 with CCR). The CCR marker adds
  32 to 47 bytes, so on a run that already prints 85 bytes it makes the output bigger.
- **Where Downshift loses outright:** `git status` (570 bytes, passed through as an unknown shape, against 177
  with all six files named by RTK), and every passing summary.
- **Where Downshift is ahead:** directory listings, `find`, `grep` and `git log`: 79.9% against 45.6%. RTK's
  `grep -rn` saved nothing. The price is that CCR folds the middle of the output. After `ls -la` only 6 of 151
  names stay visible and the agent has to call `downshift retrieve`. RTK's `ls` also truncates (50 of 151 names)
  and points to `rtk recall`.
- **Exit codes:** preserved by both tools in all 15 runs, for all three variants.

## Method

| | |
|---|---|
| RTK | 0.49.0, commit `1cf7bc703efa5cd0530222746ce40bd5d9b38336`, built from source with `cargo build --release`, default features (full edition). No hook installed. |
| Downshift | Built from this repository at the commit before the one that adds this page (CCR on by default, retrieved bytes subtracted). |
| Machine | One Linux container, Go 1.24.7 toolchain, pytest 9.1.1, cargo 1.97.0, Node 22.23.4, vitest 2.1.9, git 2.43.0. |
| Output | stdout and stderr merged, as a coding agent's shell tool shows them. Bytes of that text. |
| RTK command | What `rtk rewrite "<command>"` returns, i.e. what its hook would run. All 15 commands were rewritten. |
| Downshift command | `downshift compact -- <command>`, with CCR on (nothing set) and with `DOWNSHIFT_CCR=0`. |
| Isolation | A separate home and state directory per tool. Nothing touched a real profile. |
| Fixtures | Built by [make_fixtures.sh](compact-vs-rtk-2026-11-07/make_fixtures.sh): 12 Go packages, a Go project with failures and a panic, a pytest file with 40 passes and one with 4 failures, two Rust crates, a TypeScript project with 4 type errors, vitest files, this repository with a staged rename, 2 edits and 2 untracked files, and 190 small files. |
| Facts | For each run, a fixed list of things the agent needs: failing test names, messages, `file:line` locations, file names, commit hashes and subjects, the pass count. A fact counts as kept when its text is in the output. Lines were not compared, because RTK writes `[FAIL] TestName` where Go writes `--- FAIL: TestName`. |
| Recovery | When an output names an id (`downshift retrieve <id>`, `rtk recall <id>`), the harness ran it and counted the facts again. |

This page was measured twice. The first run was on the repository history before it was rewritten for v0.9.0 and gave 72.0% (Downshift, CCR on) against 55.5% (RTK) over all 15 runs. It was repeated on the published history; the cases that read this repository (`git status`, `git log`) changed size, and every other case is within a few bytes. The numbers here are from the second run.

The first scoring pass compared literal failure lines and marked RTK down for format alone. It was replaced by the
fact list above before any number on this page was kept. Each cell is one run. Byte counts move by a few bytes
between runs because of timings and paths.

## Results

Bytes of agent-visible output. "Facts in context" is facts kept in the output itself, before any recovery call.

| Case | Exit | Raw | RTK | Downshift, CCR on | Downshift, CCR off | Facts in context: RTK / DS on / DS off |
|---|--:|--:|--:|--:|--:|---|
| go-test-pass-12pkg | 0 | 279 | 34 | 201 | 169 | 1/1 / 1/1 / 1/1 |
| go-test-fail | 1 | 1271 | 653 | 1271 | 1271 | 13/15 / 15/15 / 15/15 |
| pytest-pass-40 | 0 | 99 | 27 | 99 | 99 | 1/1 / 1/1 / 1/1 |
| pytest-fail | 1 | 1903 | 802 | 1903 | 1903 | 12/12 / 12/12 / 12/12 |
| cargo-test-pass | 0 | 824 | 40 | 131 | 85 | 1/1 / 1/1 / 1/1 |
| cargo-test-fail | 101 | 5441 | 4607 | 4997 | 4951 | 12/12 / 12/12 / 12/12 |
| tsc-errors | 1 | 311 | 392 | 311 | 311 | 9/9 / 9/9 / 9/9 |
| vitest-fail | 1 | 1849 | 3205 | 1622 | 1576 | 6/6 / 6/6 / 6/6 |
| vitest-pass-12 | 0 | 382 | 18 | 141 | 95 | 1/1 / 1/1 / 1/1 |
| git-status | 0 | 570 | 177 | 570 | 570 | 6/6 / 6/6 / 6/6 |
| git-diff | 0 | 3208 | 3132 | 3208 | 3208 | 5/5 / 5/5 / 5/5 |
| git-log-30 | 0 | 17359 | 8747 | 2058 | 2012 | 60/60 / 60/60 / 60/60 |
| ls-la-150 | 0 | 8137 | 1139 | 536 | 8137 | 50/151 / 6/151 / 151/151 |
| find-md-40 | 0 | 591 | 287 | 185 | 453 | 40/40 / 5/40 / 40/40 |
| grep-todo | 0 | 6066 | 6066 | 651 | 6066 | 190/190 / 15/190 / 190/190 |

| Class | Runs | Raw | RTK | Downshift, CCR on | Downshift, CCR off |
|---|--:|--:|--:|--:|--:|
| failing | 5 | 10775 | 9659 (10.4%) | 10104 (6.2%) | 10012 (7.1%) |
| passing | 4 | 1584 | 119 (92.5%) | 572 (63.9%) | 448 (71.7%) |
| listing and history | 6 | 35931 | 19548 (45.6%) | 7208 (79.9%) | 20446 (43.1%) |
| all | 15 | 48290 | 29326 (39.3%) | 17884 (63.0%) | 30906 (36.0%) |

After the recovery call every fact was present for every output that named an id, for both tools.

## Where Downshift loses

1. **Failing Go, pytest and tsc runs are not compacted.** The exit code is non-zero, so Downshift passes the run
   through (`nonzero_exit`). That is the design: failures stay byte for byte. RTK saves 49% on the Go failure and
   58% on the pytest one while keeping every test name, message and `file:line` for them.
2. **`git status` was not recognised** (`unknown_shape`) for a status with a staged rename, edits and untracked
   files. RTK printed the same six files in 177 bytes. `git diff` also passed through (`danger_word`); RTK saved 2%.
3. **Passing summaries are larger.** 169 bytes against 34 (Go), 99 against 27 (pytest), 85 against 40 (cargo),
   95 against 18 (vitest). With CCR on each Go, cargo and vitest summary grows by 32 to 47 bytes for the marker.
4. **Path-qualified runners are refused.** `downshift compact -- ./node_modules/.bin/vitest run` answers
   "not allowlisted" and does not run. `npx vitest run` works. RTK rewrote both forms.
5. **`git log` is lossy.** Hashes and subjects of all 30 commits stay, but bodies, authors and dates are gone. With
   CCR off there is no way back. RTK keeps authors and relative dates and cuts bodies with a count.

## Where RTK loses

1. **Larger than the raw output** on the `tsc` and `vitest` failures. For `tsc` it adds a summary and a code count
   to four error lines. For `vitest` it keeps Node stack frames from `node_modules`.
2. **Drops stack frames in user code** in the Go panic (2 of 15 facts), recoverable with `rtk recall`.
3. **No saving on `grep -rn`** in this version (6066 bytes in, 6066 out).

## What this does not show

- Fixtures are small and synthetic. Most outputs are under 2 KB, where a 30-byte marker matters. Real
  sessions produce larger and noisier output, and the ranking on those is unknown.
- One run per cell, one machine, one version of each tool, RTK without its hook.
- Bytes, not tokens, and no check that an agent solves tasks as well with either output.
- Only commands both tools claim. RTK's `read`, `smart`, `summary` and `discover`, and everything Downshift does
  besides compaction (routing, workers), were not compared.
- The fact check is a substring match on a fixed list. It cannot see a fact nobody listed.

## Reproduce

```bash
sh docs/evidence/compact-vs-rtk-2026-11-07/make_fixtures.sh "$SCRATCH" "$PWD"
python3 -I docs/evidence/compact-vs-rtk-2026-11-07/run_bench.py "$SCRATCH" /path/to/rtk /path/to/downshift
```

Needs `go`, `pytest`, `cargo`, `node` with npm access, and `git` on PATH, and an `rtk` built from the commit above.
Captured outputs and `results.json` land in `$SCRATCH/out`.
