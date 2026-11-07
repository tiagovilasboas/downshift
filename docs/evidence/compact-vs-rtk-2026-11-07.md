<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# `downshift compact` versus rtk, head to head — 2026-11-07

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

The real command was run raw, under `rtk`, and under `downshift compact --`, on one machine, and the output bytes and the
facts that survive were compared. This is the comparison that [bench.md](../reference/bench.md#rtk-column) said had
never been done. It measures output bytes and fact survival, not model tokens, cost or task outcomes. Sample: 20
commands, 8 of them failing runs, 3 repetitions each, one machine, one day.

## Verdict

Downshift does not beat rtk on this sample. Read the table, not the headline.

- **Bytes.** Summed over the 20 commands, rtk cut 114,971 bytes to 56,556 (50.8%). Downshift's configuration at `b4acf99` cut
  them to 65,737 (42.8%), and to 57,809 (49.7%) with full CCR on, which is now the default. Against the old default rtk emitted fewer bytes
  than Downshift on 15 of 20 commands (16 with rtk's recall hint off), Downshift on 5 (4 with the hint off). With full CCR on
  the split is the same, 15 to 5, on the same five commands.
- **Where Downshift loses.** It left 11 of 20 commands within one byte of the raw output (rows 3, 5, 6, 7, 8, 9, 11, 12,
  15, 16, 18), including `pytest -v` passing (10,047 bytes; rtk 28), `git status` (671; rtk 267), `git diff` (14,697; rtk
  11,788) and `ls -la` (1,865; rtk 606). On passing runs rtk's summary is 19 to 40 bytes where Downshift's is 86 to 369
  (rows 1, 2, 10, 13), or untouched (rows 6, 7).
- **Where rtk loses.** It cut facts Downshift kept. On the failing runs, Downshift showed every fact of the raw output
  on 8 of 8; rtk showed every fact on 5 of 8 and dropped the frame in the test's own code from a Go panic (row 5) and
  six assertion-detail lines from the pytest failures (rows 8 and 9); both are reachable with `rtk recall`. On 3 failing
  commands (rows 3, 11, 12) rtk emitted more bytes than the raw output did: by 31, 2,812 and 68 bytes. On passing or
  listing output rtk dropped the package names of a passing Go run (rows 1 and 2), the absolute dates and most body lines
  of `git log`, and the owner, times and exact sizes of `ls -la`, with no pointer back.
- **Where Downshift also loses facts.** `git log` output goes to short hash plus subject: author, absolute date and every
  body line are gone (227 of 327 facts) and nothing points back to them unless full CCR is on. `grep -rn` cut 31 of 454
  facts with a note that says to narrow the search.
- **Exit codes.** All 20 commands kept the raw exit code under every arm (0, 1, 2 and 101 seen). No difference between the
  tools there.

## Which CCR setting the columns measure

Everything below was measured at `b4acf99`, where full CCR was opt-in (`"ccr": true` or `DOWNSHIFT_CCR=1`) and only the raw
output of a compacted failing command was kept by default (`ccr_failures`). The next commit on this branch makes full CCR
the default. So the table has three Downshift columns: **DS default** (nothing set, the old default at `b4acf99`),
**DS CCR=1** (`DOWNSHIFT_CCR=1`, which is what the new default produces) and **DS CCR=0** (`DOWNSHIFT_CCR=0`, which also
switches the failure store off). A pointer exists in DS default only on rows 4 and 14. Read the DS CCR=1 columns as the
current default behaviour; they were not re-measured after the change.

## Method

| Item | Value |
|---|---|
| Downshift | `b4acf99f3ebb885f5cbc7ef1b8e04897a0120933`, built from source with `go build ./cmd/downshift` |
| rtk | `rtk 1.1.0`, tag `v1.1.0`, commit `753a3b0e939f2e4490fc24e3a1984010f40a103c` (the default branch head that day), built with `cargo build --release --locked`. The release download host was refused by the sandbox proxy (HTTP 403), so the binary was built from the tag; no prebuilt binary was used. |
| Tools | go 1.27.3, Python 3.13.18, pytest 9.1.1, node 22.23.4, vitest 2.1.9 (pinned by the script), tsc 6.0.2, cargo 1.97.0, git 2.43.0 |
| Machine | one Linux container, 4 cores, 2026-11-07 |
| Script | [scripts/compact-vs-rtk.py](../../scripts/compact-vs-rtk.py); builds every fixture, runs everything, writes the JSON this page is made from |

How a command was run:

- **Same command, three wrappings.** Raw is the command as written. rtk runs the command that `rtk rewrite` returns for it
  (its own hook rewriter), so it is the form rtk's hook would run. Downshift runs `downshift compact -- <command>`.
- **Output is stdout and stderr together**, as an agent sees them, counted in bytes including the markers and hints each
  tool appends (`[downshift compact: ...]`, `[full output: rtk recall ...]`).
- **Each arm re-runs the command 3 times**; the bytes in the table are the median. The three runs differed by at most 2
  bytes (a timing digit), except the first raw `cargo test` run of each fixture, which also printed a one-time compile
  step (134 bytes more; the median ignores it). Exit codes were identical in all three runs of every arm.
- **Isolation.** Each arm has its own state directory (`DOWNSHIFT_STATE_DIR`, `RTK_DB_PATH`, `RTK_CONFIG_DIR`,
  `RTK_TEE_DIR`); rtk telemetry is off (`RTK_TELEMETRY_DISABLED=1`); `NO_COLOR=1`, `CI=1`; `RUST_BACKTRACE` unset.
  The repository is only read.
- **rtk's recall.** rtk also keeps the raw output of a failing run by default (sqlite store, `[full output: rtk recall <hash>]`).
  The table shows rtk with that default and with `RTK_RECALL=0`, so its hint line is not counted against it unfairly.
- **Fixtures** are small and synthetic, built by the script, except rows 17 to 20 and the sources behind rows 15 and 16, which
  come from this repository's own tree and history (public, no user data).

### What counts as a fact

Facts are read by regular expression from the raw output of each command, then looked for in each tool's output. A fact
counts as shown only if the output carries it in any form (a reformatted package name or a different count layout is a
match; a missing one is a loss). Every matcher was checked against the raw output first (all 20 commands match
all of their own facts). Failure runs:

- **go test**: each failing test name, each assertion site (`file_test.go:N`) and message, the panic message, the frame in
  the test's own code, and each package name.
- **pytest**: each failing test, each `E` line, each failure site, the failed/passed counts.
- **vitest**: each failing test, each error line, each `file:line:col`, the counts (as bare numbers, since rtk prints `PASS (40) FAIL (3)`).
- **tsc**: each diagnostic's line and column, code and message.
- **cargo test**: each failing test, each panic site, the assertion text, the summary counts.
- **Listings**: every path in `git status`; every file and changed line in `git diff`; hash, subject, body line, author and
  date of each commit in `git log`; every entry, plus the `rwx` permission string, owner, time and one exact byte size,
  for `ls -la`; every file name for `find`; every match line number and test name for `grep`. Note for `ls -la`: rtk
  prints permissions as octal, which carries the same information as `rwx`, so that one counts as lost only because the
  matcher wants the string.

A second, cruder count (failure-looking lines kept verbatim) is in the JSON. It penalises any reformatting, so it is not
used for claims. For Downshift it was complete on 7 of 8 failing runs, and the eighth missed only a `Compiling ...` progress
line.

## Bytes

Bytes of output. rtk and Downshift columns include their own markers. Exit code is the same for raw and every arm.

| # | Command | Result | Raw | rtk | rtk, recall off | DS default | DS CCR=1 | DS CCR=0 | Exit code |
|--:|---|---|--:|--:|--:|--:|--:|--:|--:|
| 1 | `go test -count=1 ./...` (30 packages) | pass | 930 | 34 | 34 | 369 | 220 | 369 | 0 |
| 2 | `go test -count=1 -v ./...` (3 packages) | pass | 14,967 | 34 | 34 | 283 | 329 | 283 | 0 |
| 3 | `go test -count=1 ./calc` | fail | 315 | 346 | 303 | 315 | 315 | 315 | 1 |
| 4 | `go test -count=1 -v ./calc` | fail | 2,175 | 346 | 303 | 563 | 563 | 517 | 1 |
| 5 | `go test -count=1 ./boom` (panic) | fail | 812 | 377 | 334 | 813 | 812 | 812 | 1 |
| 6 | `pytest` (120 tests) | pass | 519 | 28 | 28 | 519 | 519 | 519 | 0 |
| 7 | `pytest -v` (120 tests) | pass | 10,047 | 28 | 28 | 10,047 | 1,993 | 10,047 | 0 |
| 8 | `pytest` | fail | 2,410 | 611 | 568 | 2,410 | 2,410 | 2,410 | 1 |
| 9 | `pytest -v` | fail | 7,378 | 611 | 568 | 7,378 | 7,378 | 7,378 | 1 |
| 10 | `npx vitest run ok` | pass | 333 | 19 | 19 | 96 | 142 | 96 | 0 |
| 11 | `npx vitest run bad` | fail | 2,333 | 5,145 | 5,102 | 2,333 | 2,333 | 2,333 | 1 |
| 12 | `tsc -p .` | fail | 348 | 416 | 416 | 348 | 348 | 348 | 2 |
| 13 | `cargo test --offline` | pass | 1,769 | 40 | 40 | 86 | 132 | 86 | 0 |
| 14 | `cargo test --offline` | fail | 2,205 | 541 | 498 | 1,011 | 1,011 | 965 | 101 |
| 15 | `git status` | listing | 671 | 267 | 267 | 671 | 671 | 671 | 0 |
| 16 | `git diff` | listing | 14,697 | 11,788 | 11,788 | 14,697 | 14,697 | 14,697 | 0 |
| 17 | `git log -n 50` | listing | 23,320 | 14,532 | 14,532 | 3,287 | 3,333 | 3,287 | 0 |
| 18 | `ls -la` | listing | 1,865 | 606 | 606 | 1,865 | 1,865 | 1,865 | 0 |
| 19 | `find internal/execwrap -name *.go` | listing | 5,663 | 983 | 941 | 3,350 | 3,396 | 3,350 | 0 |
| 20 | `grep -rn func Test internal/execwrap` | listing | 22,214 | 19,804 | 19,721 | 15,296 | 15,342 | 15,296 | 0 |
| | **Sum of the 20** | | 114,971 | 56,556 | 56,130 | 65,737 | 57,809 | 65,644 | |
| | Saved against raw | | | 50.8% | 51.2% | 42.8% | 49.7% | 42.9% | |

The sum is dominated by five outputs (rows 2, 7, 16, 17, 20 are 85,000 of the 115,000 raw bytes), so it says little
about a typical command. Row 17's saving is also the one that deletes the most facts (below).

Reading the Downshift columns against each other:

- **CCR=1 against CCR=0** saved bytes on 2 rows (1 and 7: it folds runs of similar lines, and row 7 is a 10,047-byte pass
  that stays raw otherwise), cost the 46-byte marker on 8 rows (2, 4, 10, 13, 14, 17, 19, 20), and changed nothing on the
  other 10.
- **DS default against CCR=0** differs only on rows 4 and 14, where a compacted failing run gets the retrieve pointer
  (+46 bytes).
- Row 1 with CCR=1 shows 5 of 30 package names and folds the rest to `[... N similar lines folded]`; they come back with
  `downshift retrieve`.

## Facts

Facts shown in the output itself, then facts reachable using the pointer the tool prints (`rtk recall <hash>`,
`downshift retrieve <id>`). `no pointer` means the output carried no pointer, either because nothing was cut or because
nothing was stored; read it against the first column. A shown count below the facts in raw with `no pointer` is a loss the
output alone cannot undo.

| # | Command | Facts in raw | rtk shown | rtk with recall | DS default shown | DS default with retrieve | DS CCR=1 shown | DS CCR=1 with retrieve |
|--:|---|--:|--:|--:|--:|--:|--:|--:|
| 1 | `go test -count=1 ./...` | 30 | 0 | no pointer | 30 | no pointer | 5 | 30 |
| 2 | `go test -count=1 -v ./...` | 3 | 0 | no pointer | 3 | no pointer | 3 | 3 |
| 3 | `go test -count=1 ./calc` | 11 | 11 | 11 | 11 | no pointer | 11 | no pointer |
| 4 | `go test -count=1 -v ./calc` | 11 | 11 | 11 | 11 | 11 | 11 | 11 |
| 5 | `go test -count=1 ./boom` | 4 | 3 | 4 | 4 | no pointer | 4 | no pointer |
| 6 | `pytest` | 1 | 1 | no pointer | 1 | no pointer | 1 | no pointer |
| 7 | `pytest -v` | 1 | 1 | no pointer | 1 | no pointer | 1 | 1 |
| 8 | `pytest` (fail) | 25 | 19 | 25 | 25 | no pointer | 25 | no pointer |
| 9 | `pytest -v` (fail) | 25 | 19 | 25 | 25 | no pointer | 25 | no pointer |
| 10 | `npx vitest run ok` | 1 | 1 | no pointer | 1 | no pointer | 1 | 1 |
| 11 | `npx vitest run bad` | 11 | 11 | 11 | 11 | no pointer | 11 | no pointer |
| 12 | `tsc -p .` | 7 | 7 | no pointer | 7 | no pointer | 7 | no pointer |
| 13 | `cargo test --offline` (pass) | 1 | 1 | no pointer | 1 | no pointer | 1 | 1 |
| 14 | `cargo test --offline` (fail) | 9 | 9 | 9 | 9 | 9 | 9 | 9 |
| 15 | `git status` | 19 | 19 | no pointer | 19 | no pointer | 19 | no pointer |
| 16 | `git diff` | 97 | 97 | no pointer | 97 | no pointer | 97 | no pointer |
| 17 | `git log -n 50` | 327 | 237 | no pointer | 100 | no pointer | 100 | 327 |
| 18 | `ls -la` | 37 | 33 | no pointer | 37 | no pointer | 37 | no pointer |
| 19 | `find internal/execwrap -name *.go` | 149 | 51 | 149 | 149 | no pointer | 149 | 149 |
| 20 | `grep -rn func Test internal/execwrap` | 454 | 424 | 454 | 423 | no pointer | 423 | 454 |

### Facts rtk lost

- **Row 5, Go panic.** rtk keeps the panic message and the first two `testing.go` frames, and drops the frame in the test's own
  code (`boom_test.go:9`), the line that says where the bug is. It is reachable with `rtk recall` (4 of 4).
- **Rows 8 and 9, pytest.** The first assertion line and the failure site survive; the diff body (`Differing items:`,
  `Full diff:`, the changed element) is replaced by `… +12 more E lines`, 6 of 25 facts. Reachable with `rtk recall`.
- **Rows 1 and 2, passing Go runs.** The summary is `Go test: 60 passed in 30 packages`. The count is right; the package
  names are gone and no pointer is printed. A passing run needs few of those facts, but they are lost.
- **Row 17, `git log`.** Absolute dates are replaced by "N minutes ago", bodies are cut after a few lines
  (`[+3 lines omitted]`), 90 of 327 facts, no pointer.
- **Row 18, `ls -la`.** Output is octal mode, name and a rounded size (`12.3K`, `310B`). Owner and group, link count and
  modification time are gone and the exact size is rounded; the `rwx` string is re-encoded as octal, which loses nothing.
  The matcher counts 4 of 37 as lost, of which 3 are real. No pointer.
- **Row 19, `find`.** 51 of 149 names shown; the rest behind `rtk recall` (149 of 149).
- **Row 20, `grep`.** 424 of 454 facts shown, a count of what is hidden, and `rtk recall` for the rest (454 of 454).
- **Rows 3, 11, 12, bytes.** rtk's `go test` failure output (+31 bytes) is the raw output plus a 43-byte recall hint (303 bytes
  with the hint off). Its vitest failure output repeats about ten stack frames inside `node_modules` per failure, which
  raw vitest does not print: 24 of its 39 lines. Its `tsc` output keeps all four diagnostics and adds a two-line summary
  (416 against 348 bytes). The fixture's long paths inflate the vitest figure; the extra frames would exist at any path
  length.

### Facts Downshift lost, and what it kept

- **Failing runs, all 8.** Every fact was shown on every failing run. The mechanism is in the source: a failing run is
  either returned raw or has only the lines of passing tests cut (`internal/compressor/failed_run.go`), and a passing test
  that logged keeps its header lines (`internal/compressor/go_test_failed.go`). Rows 4 (2,175 to 563) and 14 (2,205 to
  1,011) are the two failing runs it shortened; row 14 kept all failure text, and row 4 kept all 11 facts.
- **Row 17, `git log`.** 50 commits of 23,320 bytes become hash plus subject, 3,287 bytes. Author, absolute date and body
  lines are gone (227 of 327 facts) with no pointer by default. With CCR=1 the marker adds a retrieve id and all 327 are
  reachable. This is the largest single saving in the table and the largest single loss.
- **Row 20, `grep`.** 31 of 454 facts cut, announced in the output as `... 4 more in this file` and `23 more matches in 7 more
  files (narrow the search, or add -l / -c)`. With CCR=1 the same output is 46 bytes longer and 454 of 454 are retrievable.
- **Row 1 with CCR=1.** To get 220 bytes instead of 369 it hides 25 of 30 package names behind `[... N similar lines folded]`.
  They are retrievable; they are not in the context the model reads.
- **Row 2.** Downshift's `go test -v` summary counts top-level tests (3 passed), rtk counts every subtest (123 passed);
  neither is wrong, they count different things.

## Why Downshift left 11 commands untouched

Three different reasons, with the weight of evidence stated for each. The probes below are ad hoc checks on the same
machine and day. They are not in the table and not in the script.

1. **Little or nothing to cut from a failing run** (rows 3, 5, 8, 11). A failing run keeps every failure line and only
   the lines of passing tests are cut (`internal/compressor/failed_run.go`), and the compressor returns the raw output
   when what it would emit is not shorter (`failed_run_no_gain`). Rows 3, 5 and 11 print no passing lines at all. Row 8
   has 60 passing tests; a probe with 500 passing tests plus one failure compacted from 1,368 to 958 bytes, and one with 30
   stayed raw at 806. Row 12, `tsc`, has no failed-run compactor: other families' failing output is returned raw by design
   (`failedFormat` in `internal/execwrap/expected_format.go`).
2. **Output outside the grammar of the pytest compressor** (rows 6, 7, 9). Read from source: `rePytestPlainLine` in
   `internal/compressor/pytest.go` lists the line shapes of a clean run, and one unlisted line keeps the whole transcript
   raw. Row 6's 120 tests wrap the progress line at 80 columns and the wrapped line has no file name in front. A probe is
   consistent with that: a 30-test and a 60-test file, whose progress line does not wrap, compacted from 421 to 81
   bytes. Rows 7 and 9 use `-v`, which prints `collecting ... collected N items` and one `nodeid PASSED` line per test,
   shapes the grammar does not list; a 30-test file under `-v` stayed raw at 2,829 bytes while rtk gave 27. So a common
   suite size (somewhere above 60 tests in one file) and a common flag (`-v`) take Downshift's pytest compaction out of
   play. The cause is read from the grammar and fits the probes; it was not confirmed with an instrumented run.
3. **Not compacted at all** (rows 15, 16, 18: `git status`, `git diff`, `ls -la`). The output came back identical to the raw
   output. This run did not find out why. rtk shortens all three (60%, 20% and 68%); only `ls -la` loses facts doing so.

## What this does not show

- **Sample.** 20 commands, 8 failing, 3 repetitions, one machine. The repetitions test timing noise, not different inputs.
  Nothing here is a rate across projects, and no confidence interval is claimed.
- **Fixtures are small.** Many failing runs print 300 to 2,500 bytes, so a 43-byte hint or a 46-byte marker is a visible
  share of them. Real failures with long logs and stack traces are where both tools have more to cut, and the ranking
  may differ there.
- **Synthetic input shape matters to CCR.** An earlier pilot with a diff of near-identical lines (not kept, not
  reproducible with the script) folded a `git diff` from 18,712 to 2,065 bytes with most changed lines hidden; with
  realistic varied code lines, as in row 16, CCR=1 folded nothing. Treat CCR=1's gains as dependent on the output's shape.
- **Bytes are not tokens or outcomes.** No model read these outputs. Whether a model fixes a failure faster from rtk's
  summary or from Downshift's near-raw output is the question [benchmark-output-cutting.md](../guides/benchmark-output-cutting.md)
  asks and this page does not answer.
- **Fact matchers are mine.** They were checked against raw output and corrected twice during the run (a literal `FAIL`
  line, full import paths, a literal `3 failed`), and every reported loss was read in the output. A different fact list
  could move a count by a few.
- **rtk covers 100+ commands; 11 command families are measured here.** Nothing is said about the rest.
- The older rtk numbers in [bench.md](../reference/bench.md#rtk-column) replay fixtures through `rtk log` and remain not
  like-for-like. This page is the like-for-like run that section asked for, for these commands.

## Reproduce

```bash
python3 scripts/compact-vs-rtk.py --downshift /path/to/downshift --rtk /path/to/rtk --work /path/to/empty-dir
```

The script needs `go`, `pytest`, `node`/`npm` (vitest is installed into the work directory), `tsc`, `cargo` and `git`
on PATH, and builds every fixture there. It refuses a non-empty work directory, reads this repository without writing
to it, and prints the same numbers as the tables above (up to one byte of timing). The `git log`, `ls`, `find` and `grep`
rows depend on this repository's history and tree: they match this page at commit `b4acf99`.
