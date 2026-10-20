# Tokens only: remove dollar figures from the product

Owner decision (2026-10-20): Downshift reports the token-saving quantity, not
dollars. Dollars stay only where the accounting needs them, and are removed from
every output in small, separately shipped parts, highest value first.

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

## Parts

| # | Part | Value | Risk | Status |
|---|------|-------|------|--------|
| 1 | `downshift report --json`: drop dollar keys (`cost_usd`, `cost_per_completed_task_usd`, `per_task_*_usd`, `savings`, `prices_as_of`). Go fields stay, `json:"-"`. | high (what users read) | low: one output | in progress |
| 2 | Text report and docs already show tokens only | high | done | done (`63fdfdc4`, `95deac3f`, `ff53579b`) |
| 3 | `downshift stats --export`: drop `*_usd` keys. This is the operator contract (issue #104): bump the export version, update `docs/reference/stats-export.md` and the `simbench` invariants that read those keys. | high (external operators) | medium: contract | done |
| 4 | Event log: stop writing `actual_cost_usd` / `baseline_cost_usd`. Keep reading old lines. Summaries (`GrossSavedUSD`, `NetSavedUSD`, overspend) move to tokens. | medium | high: changes accounting history | not started |
| 5 | Remove the catalog-price valuation from report and stats (`RepriceUsage`). Routing keeps using the catalog. | low | medium | not started |

## Rules for every part

- One part per branch and PR, based on `main`. Auto-merge on, per the owner's rule for personal repos.
- Tests: change only the assertions about the removed output. Accounting tests (`cost_test.go`, `summary_test.go`, `simbench`) keep their dollar math until part 4.
- Verify with `go test ./cmd/downshift/ ./internal/telemetry/ ./internal/simbench/` and `downshift report --json`.
- Model review of each diff: DeepSeek V4.1 Flash via `cheap_models/cheapllm.py` (text only, no edits).
