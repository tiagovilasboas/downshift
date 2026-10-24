<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# Claude Code at 100%, then command UX (2026-10-23)

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

Owner's order: first make model routing and the Compactor fully work in Claude Code, then improve
the command UX. Two read-only spikes ran on `origin/main` against a copy of the state dir.
Continues [`../2026-10-23-spike-progress-kiro-accounting`](../2026-10-23-spike-progress-kiro-accounting/README.md).
Each task is one PR of at most 5 story points.

## 1. Claude Code today: wired, but saves little

- Wiring is correct: the plugin's PreToolUse `Agent|Task` and `Bash`, PostToolUse, SubagentStop and
  UserPromptSubmit hooks call the installed binary; `doctor --json` has no failure; 5 of 5 rewrites
  were honored.
- Routing, last 3 days: 72 decisions; 5 changed the model (opus → sonnet), 8 changed only the
  effort, 16 upshifts and 6 downshifts were held. **42 of 72 decisions were
  `CLASSIFIER_UNCONFIDENT`**, so most spawns stay as requested. A parent that asks for haiku caps
  the child at haiku (routing never goes above the session model), which is why a session shows
  effort-only changes. That cap is the documented design, not a bug.
- Compactor: the Bash rewrite fires live (84 hook entries since 2026-10-22), but only 5 compacted.
  580 of 982 uncompacted runs printed 200 bytes or less (mostly git); 108 were refused direct calls
  (exit 126); failed runs pass through in full by design (223 in 3 days, 44 large `go test`).
  `FOO=1 go test ./...` is not rewritten. Overall the Compactor cut about 1.4% of the bytes.

## 2. Claude Code tasks

| # | Task | Pts | Files | Decision |
|---|------|--:|-------|----------|
| C1 | Classifier calibration: add table tests for the unconfident shapes that have a clear tier, and lower `CLASSIFIER_UNCONFIDENT` without raising wrong picks | 5 | `internal/classify`, `classifier_edge_test.go` | — |
| C2 | Exec events carry a pass-through reason (`small_output`, `nonzero_exit`, `not_allowlisted`, `metachar`, `env_prefix`), so the Compactor breakdown is measurable | 2 | `internal/execwrap` run and event | — |
| C3 | Skip the rewrite for families whose output stays below the threshold (git readouts), so the Compactor wraps only commands it can cut | 3 | `internal/execwrap` policy | after C2 data |
| C4 | Accept a leading `VAR=value` assignment in the rewrite (`FOO=1 go test ./...`) | 2 | `internal/execwrap` policy | — |
| C5 | Compact failed runs: keep every failure and its context, cut only the passing lines | 5 | execwrap family compressors | **owner**: today failures pass through in full on purpose |
| C6 | Explicit parent model and upshift: allow an upshift above an explicitly requested model, or say the cap in the panel | 3 | core plan, `session-models.md` | **owner** |

## 3. Command UX (after section 2)

Findings: `stats --help` fails (exit 2); `compact` is missing from `help`; `report` prints 36 lines
and its answer is the last one; `stats` and `report` show different "saved" figures; an unknown
command dumps the whole help; terms like held, control and baseline are not explained; `stats`
has no `--json`; `doctor` prints 150-character absolute paths. Guidelines: clig.dev (help,
output, errors, subcommands, next steps).

| # | Task | Pts | Files | Contract |
|---|------|--:|-------|----------|
| U1 | `--help`/`-h` on every command, with `Examples:` and a next-step line | 3 | `cmd/downshift/usage.go`, `help.go` | no |
| U2 | Unknown command: one error line, "Did you mean …?", "Run downshift help" | 1 | `cmd/downshift` main | no |
| U3 | `compact` in `help`; `shell-init` a documented deprecated alias; `feedback` without args prints usage | 2 | `help.go`, feedback | no |
| U4 | `report` leads with a 3-line summary (the `Numbers` display strings), details behind `--detail` | 3 | `report.go` | text only |
| U5 | Plain words in the `session` header and rows; codes only in `--why` | 2 | session tree render | text only |
| U6 | `doctor`: home-relative paths and `--quiet` | 2 | doctor | no |
| U7 | `stats` becomes `report --days` (accounting-v1 D6), `--export` kept as a deprecated alias for one release | 5 | `stats`, `report` | **yes**, flags |
| U8 | `--json` on `models list` and `feedback`; `--period` alias of `--days` | 3 | those commands | new flags only |

## 4. Order

- Now, in parallel: C1, C2, C4.
- Then C3 (reads C2's reasons). C5 and C6 wait for the owner.
- Then U1, U2, U3, U5, U6 in parallel; U4 after R1 and R2 of the previous plan; U7 last; U8 any time.

Total: 46 points, 14 tasks.
