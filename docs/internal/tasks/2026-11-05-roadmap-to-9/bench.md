<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# BN: Compactor saving regression guard

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

Part of the [roadmap to 9](README.md).

## Problem

Nothing fails when the Compactor's saving drops or a lossless family starts
dropping content.

- `make bench` (Makefile:115-117) runs `TestBenchReport`. That test is skipped
  unless `DOWNSHIFT_BENCH_WRITE=1` is set, and it asserts nothing. It only
  writes `docs/reference/bench.md`.
- `DOWNSHIFT_BENCH_BASELINE=1` writes `testdata/bench/baseline.txt`, a single
  line (`48.9`). No test reads that file, and no Makefile target, CI step or
  doc mentions the variable.
- "Lossless" means only "not in `lossyFamilies`"
  (`internal/compressor/bench_report_test.go`, around line 126). Nothing
  proves it.
- Two runs in a row produced different `bench.md` output. The CCR column moved
  (eslint 1940 to 1938 bytes). The cause is not found.
- The repo has no `.gitattributes`, so a Windows checkout may turn the fixtures
  into CRLF and shift the byte counts.
- The `test-os` job (ci.yml:268, ci-full.yml:41) does not run
  `internal/compressor`.

Measured on main (2026-11-05):

- 78 compacted fixtures.
- 101,735 bytes raw, 52,029 bytes out: 48.9% saved.
- Lossless families: 69.4% saved over 44 fixtures. Lossy families: 38.0% over
  34 fixtures.
- `make bench` takes 2.9 s in wall time; the test body takes 0.85 s.

The corpus is mostly synthetic (corpus_test.go:5), so these are regression
numbers, not real-world savings.

## Tasks

| ID | Title | Pts | Depends on | Phase |
|----|-------|-----|------------|-------|
| BN-1 | Make the bench deterministic and pin line endings | 2 | none | 1 |
| BN-2 | Fixture coverage report per family | 2 | none | 1 |
| BN-3 | Committed baseline table and a failing floor test | 5 | BN-1 | 2 |
| BN-4 | Lossless invariant per family | 3 | none | 2 |
| BN-5 | Runner failure-summary guard from the MANIFEST | 3 | BN-2 | 2 |
| BN-6 | Run the guards on Windows and macOS | 1 | BN-1, BN-3, BN-4 | 3 |
| BN-7 | Second fixture for single-sample families | 5, split into PRs of at most 3 | BN-2, BN-3 | 3 |
| BN-8 | Document how to update the baseline | 1 | BN-3 | 3 |

### BN-1: Make the bench deterministic and pin line endings (2)

**Design**

- Find what moves the CCR column: marker length, fold, or a stale committed
  doc. Fix it, or exclude the column from every guard.
- Add `.gitattributes` with `internal/compressor/testdata/** -text`. Fixtures
  that are CRLF on purpose keep their bytes.

**Files**

- `.gitattributes` (new)
- `bench_report_test.go`
- `docs/reference/bench.md`
- possibly `repeat_fold.go`

**Tests**

- Two bench runs give byte-identical output.
- A test asserts that no fixture contains an unintended CR.

**Acceptance**

- Running `make bench` twice leaves `git diff` empty.

### BN-2: Fixture coverage report (2)

**Design**

- A deterministic report of clean and preserve fixtures for each compressor
  `Format` and each execwrap family, marked 0, 1 or 2+. It is informational
  only and fails nothing.
- A manual count shows about 40 families with one fixture, among them aws,
  docker, terraform, cargo_*, dotnet, git push/pull/fetch, gh_pr, ls and rg
  `file:line:col`.

**Acceptance**

- The report lists every family.

### BN-3: Baseline table and floor test (5)

**Design**

- Replace `baseline.txt` with `testdata/bench/baseline.tsv`: family, fixtures,
  raw bytes, reduced bytes, all as integers.
- `TestBenchGuard` is never skipped. It runs `benchCorpus` with rtk off and the
  CCR column excluded. It fails when:
  - a family's saving falls below its baseline minus a tolerance (about 2
    points, or a byte floor for families with one fixture);
  - a family has fewer fixtures than its baseline;
  - a baseline family disappears;
  - a family compacts but is missing from the baseline;
  - the lossless, lossy or total figures drop.
- Compare integers, never the printed `%.1f`.
- `make bench-baseline` rewrites the table. The failure message names the
  family, its old and new saving, and that command.
- The guard lives in its own `bench_guard_test.go` to respect the file-size
  rule.

**Tests**

- Table-driven checks of the comparison: drop, equal, new family, vanished
  family, tolerance edge.
- The real guard runs against the committed baseline.

**Acceptance**

- Cutting a compressor's saving by more than the tolerance fails
  `go test ./internal/compressor`.
- A baseline change shows as a reviewable diff.

### BN-4: Lossless invariant (3)

**Design**

- `map[Format]invariant` checks that every signal line of the input survives in
  the output:
  - grep_group: every `file:line` and its match;
  - runners: FAIL, panic, error and the pass/fail totals.
- It runs over the clean MANIFEST fixtures of each lossless family. A lossless
  family with no invariant fails, unless it is in an explicit allowlist.

**Tests**

- The helper fails on a deliberately lossy output.

**Acceptance**

- Dropping a grep match or a failure line fails CI.

### BN-5: Runner failure-summary guard (3)

**Design**

- Every runner format with a clean fixture needs at least one preserve (failing)
  fixture.
- That fixture stays byte-identical at every non-zero exit, and a failing body
  at exit 0 is not summarised away.
- It reuses `rawFixtures` and `nonZeroExits`.

**Acceptance**

- Removing a runner's failing fixture fails the test.

### BN-6: Cross-OS run (1)

**Design**

- Add `./internal/compressor/ -run 'BenchGuard|Lossless|CorpusRaw'` to the
  Windows and macOS steps of `ci.yml` and `ci-full.yml`. It adds about 5 s.

**Acceptance**

- Green on windows-latest and macos-latest.

### BN-7: Fixture gaps (5, split)

**Design**

- Add a second realistic fixture to the highest-use single-sample families
  from BN-2, refreshing the baseline in the same PR.
- Fixtures are sanitized and labelled synthetic when they are not real, and
  hold no secrets, names or paths.

### BN-8: Baseline update path (1)

**Design**

- Write a short section in `docs/contrib`: `make bench-baseline`, and the rule
  that a lowered baseline needs a stated reason.

## Owner decision

**BN-D1.** Can a family's floor go down with a reason only in the PR text, or
must it also be recorded in the CHANGELOG?

Recommendation: both. The CHANGELOG line is the audit trail once the PR closes.
