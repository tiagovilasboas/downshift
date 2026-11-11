<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# Impact roadmap: close the gap between the bench and real use

Owner direction, 2026-11-10. Downshift by Tiago Vilas Boas\
https://github.com/tiagovilasboas/downshift-ai

Follows the [value roadmap](value-roadmap.md). It comes from a code review of
the whole repository on 2026-11-10 and from one maintainer's 30-day exec log.

## Why

The engineering is solid: hooks fail open, core packages have 80% to 100% test
coverage, and CI runs on three operating systems. The measured effect in real
use is small, and the review found where it leaks.

| Measure | Value | Source |
|---|---|---|
| Runs through the Compactor in 30 days | 4,449 | maintainer log, 2026-11-10 |
| Of those, compacted | 499 (11%) | same |
| Bytes cut | 4.6% (1.33 MB of 28.8 MB) | same |
| Runs with no recorded reason for staying raw | 213 | same |
| Runs counted as harness `unknown` | 1,177 | same |
| Codex runs through the hook | 0 of 784 | same |
| An output over 2 MB | passes whole: a 13.2 MB `git grep` printed 13.2 MB | built binary, 2026-11-10 |
| One stderr line outside 12 families | turns compaction off: a 122 KB `make` run printed 122 KB | same |
| Process start, paid by every hook call and every wrapped command | 44 ms; 20 ms is the classifier's regexes, 6.6 ms the compressor's | `GODEBUG=inittrace=1` |
| Claude Code `PostToolUse` accepts `updatedToolOutput`, which replaces the output of built-in tools (Bash, Read, Grep, Glob) | Downshift only measures there, and its docs and `doctor` say a hook cannot rewrite tool output | hooks reference, read 2026-11-10 |
| Commands the hook leaves unwrapped (pipes, redirects, long commands) | never compacted: 7,769 skips in five days in the value roadmap's log | [value roadmap](value-roadmap.md) |
| Published figures | 68.3% on a synthetic corpus; routing 24.1% from a study that predates decision O8 | README |
| Size | 89k lines of Go, 88 packages, about 38 commands, 248 documentation files | repository |

## Already shipped on 2026-11-10

These came out of the same analysis and are on `main`.

| PR | Change |
|---|---|
| #58 | `report` shows what the agents pane shows; `--daily` and `--history` subtract read-back bytes and count runs |
| #60 | Diffs compact when a failure word sits on a line that is kept anyway |
| #62 | Balanced compaction, on by default: an output over 16 KB is cut once CCR stored it |
| #63 | `--harness=<name>` on every wrapped command and in the steering, so runs stop landing in `unknown` |
| #64 | Report screen redesigned as one block per harness |
| #65 | Balanced benchmark on four large real outputs: 76.3% fewer bytes |
| #66 | Balanced line cut never splits a multi-byte character |
| #68 | Task H1: the classifier's tables are built on first use; the Bash hook went from 36 ms to 20 ms |

## Phases

Points are story points (1, 2, 3, 5, 8). Each task is one PR that passes
`make check` on its own. A task that touches the binary fills
[coverage.md](../contrib/coverage.md) first.

### G. Compactor blind spots

The largest outputs are the ones that reach the agent whole.

| ID | Task | Pts | Done when |
|---|---|---|---|
| G1 | Output over the 2 MB hold limit: keep the head in memory and spool the rest to the CCR store instead of releasing it. Print the head, the tail and the retrieve id. Without CCR, behave as today. | 5 | A 13 MB `git grep` prints under 64 KB and `downshift retrieve <id>` returns all of it; memory stays bounded; exit code unchanged |
| G2 | Stderr in a family outside `stderrIsReport`: hold both streams in write order (as the merged families do). On a clean exit compact stdout as today and print stderr after it; on any exit apply the balanced cut when CCR stored the raw output. | 5 | A 122 KB `make` run with one warning line is cut, the warning is printed, and a run that fails keeps every failure line |
| G3 | Record why a run streamed: new passthrough reasons `too_large`, `stderr_output` and `hold_timeout`, with labels in `report`. | 2 | `report` shows no `reason not recorded` row for runs made by the new binary |
| G4 | Balanced limits in the config file (`balanced_threshold_kb`, per-kind line caps), read once at the edge. | 2 | A test sets each key; `doctor` prints the values in effect |
| G5 | A failure line longer than the line cap keeps its end as well as its start (an assertion often ends the line). | 1 | A 900-byte line with a failure word keeps its first 300 and last 200 bytes |

### H. Latency

Every Bash call pays the process start twice: the hook, then the wrapper.

| ID | Task | Pts | Done when |
|---|---|---|---|
| H1 | Build the classifier's regexes on first use, not at package init. **Done in #68.** | 2 | `inittrace` shows `internal/classify` at 1 ms; classifier tests unchanged |
| H2 | Same for the compressor's regexes that only one family uses. **Done in #70:** every package-level regexp (about 390, compressor included) compiles on first use through `internal/lazyre`; package init 14.7 ms to 1.3 ms, compressor 8.9 ms to under 0.1 ms. | 3 | `internal/compressor` init under 2 ms |
| H3 | Startup guard: a test fails when a package's init compiles more than a set number of regexes. **Done in #70:** `internal/archtest/startup_test.go` fails on any regexp compiled at package init (allowlist empty). | 2 | The guard fails on a new package-level `MustCompile` in `classify` |

### I. Proof in real use

| ID | Task | Pts | Done when |
|---|---|---|---|
| I1 | After one week on a binary with balanced on, publish the real-use cut next to the bench in the README, with the window and run count. | 1 | README states a real-use figure and how to reproduce it with `downshift report` |
| I2 | Run the routing study again on the implementation after decision O8, same protocol as the 46-run study. | 5 | A dated evidence document; the README routing row cites it or says the effect is not established |
| I3 | One source for cited figures: a test compares the numbers in README (EN/PT), `llms.txt`, true north and the evidence guide with `bench.md`. **Done in #70:** `TestCitedBenchFiguresMatchBenchMd` in `internal/docscheck`. | 3 | Changing `bench.md` without the citations fails `make check` |

### J. Harness depth before breadth

Seven harnesses are wired; one works end to end on the maintainer's machine.

| ID | Task | Pts | Done when |
|---|---|---|---|
| J1 | `doctor` for Codex: read `config.toml` for `[features] hooks = true`, and warn with the exact fix when hooks are installed and no hook event was ever recorded. | 3 | On a machine with the hook untrusted, `doctor` names `/hooks` as the fix |
| J2 | Support tiers with criteria in the harness matrix: Supported (hook verified in a real session by `doctor`), Beta, Experimental. The README table follows it. | 2 | Every harness has a tier and the evidence for it |
| J3 | `doctor --verify <harness>`: guide one real spawn and one shell command, then confirm both events arrived. | 5 | Passing output for Claude Code and Codex is attached to the PR |
| J4 | Runs from the desktop app's Code tab: find what identifies them and attribute them. | 2 | A run from that surface is not `unknown` |

### K. Weight

| ID | Task | Pts | Done when |
|---|---|---|---|
| K1 | Inventory the packages that predate decision O8 (`classify`, `semantic`, `fuse`, `sensor`, `cascade`, `loop`, `gateway`, `benchmark`): for each, who calls it and what breaks without it. The owner decides keep, mark experimental or remove. | 3 | A table in this file with a decision per package |
| K2 | Act on K1: remove what was decided, and keep the rest off the hook and wrapper path. | 5 | No hook or `compact` invocation imports a removed or experimental package |
| K3 | Hide experimental commands from the default help; list them under `downshift help --all`. **Done in #70:** `clispec.Command.Experimental`; marked `context`, `learning`, `loop`, `eval-outcome`, `probe`, pending K1's decisions. | 2 | Default help shows the daily commands only |
| K4 | Move report building and doctor checks out of `package main` into `internal` packages. | 8 | `cmd/downshift` only parses flags and prints |
| K5 | Rename `internal/discover` (it reads like `internal/discovery`, which does a different job). **Done in #70:** now `internal/missedsavings`. | 1 | `go list ./...` shows one of the two names |

### L. Release

| ID | Task | Pts | Done when |
|---|---|---|---|
| L1 | Release the work above in the version the Author names, per the [release policy](../contrib/release-policy.md). | 1 | Published release, verified on three operating systems |

### M. Replace tool output after the run

Where the harness lets a hook replace a tool's output, compaction no longer
depends on wrapping the command first. That removes the allowlist, the
metacharacter skips and the stderr problem for that harness, and it reaches
the agent's own Read, Grep and Glob, where most file content enters the context.

| ID | Task | Pts | Done when |
|---|---|---|---|
| M1 | Spike on Claude Code: answer `PostToolUse` for Bash with `updatedToolOutput` (compacted stdout, stderr kept, raw stored in CCR). Measure against the wrapper on the same commands. | 5 | A command with a pipe and a redirect is compacted and counted in `report`; a written result says whether this replaces the wrapper on Claude Code |
| M2 | Grep and Glob results: cap per file and in total, with a count of the rest, raw in CCR. | 3 | A 2,000-match Grep reaches the model under 16 KB |
| M3 | Read of a large file: never rewrite a line (Edit needs the exact text). Cut only whole ranges, with line numbers kept and a marker that names the range and the retrieve id. | 5 | An Edit after a cut Read still matches; a lockfile or generated file over 64 KB is cut |
| M4 | Correct the claim that a hook cannot rewrite tool output: `doctor`, `context status`, the context-optimization docs and the harness matrix say which harnesses allow it. | 1 | No text says it is impossible on Claude Code |
| M5 | `replaces tool output` as a capability in `HarnessCapabilities`, so other harnesses with the same channel get it from the shared layer. | 3 | The capability is data; no adapter holds the logic |

## Order

1. **Wave 1**: H1 (done), G3, M4, then the M1 spike. G1 and G2 follow M1's result: where output can be replaced after the run they may not be needed. Release L1 can follow this wave.
2. **Wave 2**: M2, M3, M5, J1, J2, I3, G4, G5.
3. **Wave 3**: K1 (needs the owner's decisions), then K2, K3, H2, H3.
4. **Later**: I1 (needs a week of data), I2, J3, J4, K4, K5.

At most three PRs open at once.

## Targets

| Target | Today | Goal |
|---|---|---|
| Bytes cut in real use, 7-day window | 4.6% | 40% or more |
| Runs with no recorded reason | 213 in 30 days | 0 on the new binary |
| Process start | 44 ms (about 5 ms after H1 to H3: `version` 4.6 ms, Bash hook 5.5 ms, median of 30 on one machine) | 20 ms or less |
| Harnesses with a `doctor`-verified hook | 1 | 2 |

Goals are checked with `downshift report` on the maintainer's log, not on the
synthetic corpus.
