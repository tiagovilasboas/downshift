# W7.8 estimate against actual: what is measured and what is not

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

**Date:** 2026-10-28
**Status:** Partial. Estimates come from the cards. **No actual effort was
recorded** (minutes, tool calls or tokens per card), so there is no actual
column and no variance. W7.8 stays open until actuals are recorded.

## Estimates from the cards

| Cards | Estimate (points) | Delivered on the branch |
|---|--:|---|
| Stage 0 (B7, B8, B9; B0 to B6 had no points) | 8 | Yes |
| Stage 2 (C0 to C4) | 14 | C1 to C4; C0 (real-session proof) open |
| W7.1 default profile table | 3 | Yes (`532bd279`, fix `fffde0a3`) |
| W7.2 policy init in setup | 2 | Yes (`50f80a0a`) |
| W7.9 economy mode | 3 | Yes (`9c1a3e62`) |

Onda 0 and Onda 1 work (F0.x, F1.x) had no point estimates on cards, so it is
not listed.

## Measured facts

- Commits `959d34b7..192296fd`: 28 on `downshift/mvp-ponto-zero`.
- Diff `959d34b7..HEAD` before this document: 62 files, +3,978 / −136 lines,
  tests included.
- `make check` (gofmt, vet on host, Windows and Linux, `go test ./...`) passed
  locally on macOS at `7c71a0a0` and at `cad01be5`. No remote CI ran on this
  branch: the workflow triggers only on pull requests and pushes to `main`.

## For next time

Record minutes or tool calls per card at close, as the harness-core delivery
rule asks, so this table can carry an actual column.
