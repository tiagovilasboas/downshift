# Tokens only: remove dollar figures from the product

Owner decision (2026-10-20): Downshift reports the token-saving quantity, not
dollars. Dollars stay only where the accounting needs them, and are removed from
every output in small, separately shipped parts, highest value first.

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

## Parts

| # | Part | Value | Risk | Status |
|---|------|-------|------|--------|
| 1 | `downshift report --json`: drop dollar keys (`cost_usd`, `cost_per_completed_task_usd`, `per_task_*_usd`, `savings`, `prices_as_of`). Go fields stay, `json:"-"`. | high (what users read) | low: one output | done (#278) |
| 2 | Text report and docs already show tokens only | high | done | done (`63fdfdc4`, `95deac3f`, `ff53579b`) |
| 3 | `downshift stats --export`: drop `*_usd` keys. This is the operator contract (issue #104): bump the export version, update `docs/reference/stats-export.md` and the `simbench` invariants that read those keys. | high (external operators) | medium: contract | done (#283) |
| 4 | Event log: stop writing `actual_cost_usd` / `baseline_cost_usd`. Keep reading old lines. Summaries (`GrossSavedUSD`, `NetSavedUSD`, overspend) move to tokens. Split in 4a to 4e below, in this order. | medium | high: changes accounting history | done (#287 to #289, #292, #293; docs #294) |
| 5 | Remove the catalog-price valuation from report and stats (`RepriceUsage`). Routing keeps using the catalog. | low | medium | done (#290) |

## Part 4 split

Why split: the direction of a reroute (`cheaper`, `pricier`, `unchanged` in `tokens_rerouted`, `token_saving.go`, the live line) is decided today by comparing `baseline_cost_usd` with `actual_cost_usd`. Stopping the writes first would blank the dock and the export. So the readers move first, the writer last. Each step is its own PR, based on `main`, green before the next.

| Step | Change | Risk | Status |
|------|--------|------|--------|
| 4a | Add a price-free direction: classify a usage record as cheaper, pricier or unchanged from the requested and ran model tiers (catalog rank), not from dollars. Same result as today on existing logs, proven by a test over `simbench` and the local log. | medium: new source of truth | done (#287) |
| 4b | `tokens_rerouted`, `token_saving.go` and the live line read the 4a direction. The `*_usd` fields are no longer read for buckets or the headline. | medium | done (#288) |
| 4c | Library summaries (`GrossSavedUSD`, `NetSavedUSD`, overspend, `RealSavedUSD` and friends) become token quantities. `simbench` invariants move with them. | high: accounting API | done (#289) |
| 4d | Stop writing `actual_cost_usd` / `baseline_cost_usd` on new usage events (`FillRealCost`, `RoutedCost`). Old lines still parse. Test: a legacy line and a new line aggregate the same. | high: history | done (#292) |
| 4e | Delete dead dollar code left by 4a to 4d (`reprice.go` stays for part 5). Docs: `docs/reference/telemetry.md` event fields. | low | done (#293 code, #294 docs) |

Files touched by the dollar fields today: `internal/telemetry/{aggregate,curation,export,reprice,session_tree,tasks,telemetry,token_saving,tokens_rerouted,usage}.go`, `internal/childend/record.go`, `internal/simbench/{invariants,result}.go`, `cmd/downshift/verification.go`.

## Rules for every part

- One part per branch and PR, based on `main`. Auto-merge on, per the owner's rule for personal repos.
- Tests: change only the assertions about the removed output. Accounting tests (`cost_test.go`, `summary_test.go`, `simbench`) keep their dollar math until part 4.
- Verify with `go test ./cmd/downshift/ ./internal/telemetry/ ./internal/simbench/` and `downshift report --json`.
- Model review of each diff: DeepSeek V4.1 Flash via `cheap_models/cheapllm.py` (text only, no edits).
