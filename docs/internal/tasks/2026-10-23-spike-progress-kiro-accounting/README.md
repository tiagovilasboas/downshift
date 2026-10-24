<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# Spike: progress bar, Agents panel, KiroCrew and accounting (2026-10-23)

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

Owner's question: after the accounting and KiroCrew work, are the progress bar, the Agents panel
and KiroCrew working? Three read-only spikes ran on `origin/main` (f52864d7) against a copy of the
state dir. Nothing was written to `~/.downshift` or `~/.kiro`. Each task below is one PR of at
most 5 story points.

## 1. Findings

### 1.1 Progress bar and Agents panel (savvy-progress): working, installed copy stale

- The contract holds: every field the mod reads from `downshift session --panel` exists with the
  same name and type after #435 to #441, #476 and #477.
- Mod suites: savvy-progress 41 of 42 pass, filetree 68 of 68. The one failure
  (`downshift.test.ts:126`) expects an older error text from the `claude plugin test` runner and
  fails the same way on 1.2.11, so accounting did not cause it. No CI job runs these suites.
- The installed plugin is 1.2.11; `main` has 1.2.12 (#425): one `Tokens saved ~N` tile instead of
  the old `Downshifted` and `Compactor total` tiles.
- The mod still adds `work_saved + cache_saved` itself (`register.tsx` around 813, 816, 821 and
  901), so the summary gate and the per-spawn label can disagree with the tile.
- The tile mixes scopes: session routing plus the all-time Compactor (`sessionpanel.go:27-33`),
  with no scope in the label.

### 1.2 KiroCrew: partial

- Wired and answering: 16 Kiro agents carry the Downshift pre and post hooks; `doctor` is green;
  real-shaped payloads route (`spawn_run` on opus → exit 2, respawn on haiku), advise `go test`
  as `downshift compact -- go test ./...` (#464 holds), and fail open on garbage.
- Claude models served by Kiro (sonnet, opus, haiku ids) are routed. Compaction depends on the
  agent following the advise; KiroCrew has no rewrite channel.
- Gaps, verified in code:
  - `isSubagentSpawn` (`internal/adapters/kirocrew/kirocrew.go:102`) never checks the tool name:
    any tool whose input has `model` and `prompt` is routed and logged as a spawn.
  - `spawn_sub_agents` with a nested `agents[]` batch is not covered (exit 0, nothing logged).
  - `childend.go:75` sets `ResolvedModel` to the requested model: a served model claimed without
    evidence.
  - The respawn message text lives in the adapter (policy in an adapter).
  - Live blocking is unproven: zero shell-advise rows and zero child-end rows in the log; the
    `acp_backend` of this machine is not the one the A-P7 note assumed.

### 1.3 Accounting: not integrated

- `docs/design/accounting.md` said "implementation complete". In code, only T0 to T3 are done:
  `internal/accounting` has no importer outside itself and `formula.Saved` has no production
  caller. The status line is corrected in this PR.
- Five savings aggregations remain (`telemetry.TokenSavings`, `Stats.TokensRerouted`,
  `Summarize`, `accounting.CalculateSavings`/`BuildGlobalSummary`, `formula.Saved`); only the first
  feeds a surface.
- Surfaces disagree for the same data: final saved estimate 15,916,879 (report, 30 days) vs
  15,121 (panel, session routing + all-time Compactor) vs 120 (dashboard API, session Compactor);
  held 3 (panel) vs 4 (API) vs 18 (stats, 30 days).
- Dashboard API (`internal/api/server.go`): counts verdict OK and UPSHIFT with `allow` as held
  (`:330-338`) and effort-only DOWNSHIFT as switched; `/api/session/{id}` uses the all-time
  Compactor while `/api/sessions` uses the session one; `handleSessions` reads only the log tail;
  `panelOut` and `compactorPanel` are copied from `cmd/downshift/sessionpanel.go`.
- Formula (`internal/accounting/formula/formula.go:121-126`): an unpriced bucket is priced at the
  baseline price instead of being excluded, and `SavedPercent` returns 0 where the spec says `n/a`.
- `archtest` has no rule for the L0 to L4 layers. Files over 300 lines: `telemetry.go` 896,
  `session_tree.go` 514, `internal/api/server.go` 391, `tokens_rerouted.go` 326, `aggregate.go`
  317, `report.go` 304.

## 2. Tasks

| # | Task | Pts | Files | After |
|---|------|--:|-------|-------|
| K1 | KiroCrew: route only `spawn_run` and `spawn_sub_agents`; table test with a non-spawn tool carrying `model` and `prompt` | 2 | `internal/adapters/kirocrew/kirocrew.go` + test | — |
| K2 | KiroCrew child end: `ResolvedModel` stays empty unless the payload names it | 2 | `internal/adapters/kirocrew/childend.go` + test | K1 |
| K3 | KiroCrew: route each entry of a `spawn_sub_agents` `agents[]` batch; post-hook matcher covers it | 3 | kirocrew adapter, setup | K2 |
| K4 | Move the respawn message builder to a shared layer; the adapter only emits it | 2 | kirocrew adapter, shared hook runner | K3 |
| K5 | Live proof in an interactive Kiro session: one shell advise row and one blocked spawn in the log | 3 | evidence note | K1; needs the owner |
| A1 | Dashboard API: held and switched from the telemetry definitions, one Compactor scope per session, full log for the session list, sort with `sort.Slice` | 3 | `internal/api/server.go` + test | — |
| A2 | Formula: exclude unpriced buckets from both costs and count them; `SavedPercent` reports `n/a` with no priced baseline | 2 | `internal/accounting/formula` | — |
| A3 | `accounting.Compute(scope)` returns `Numbers` (tokens, saved, compactor, held) from `TokenSavings`, with the display strings once | 5 | `internal/accounting` | A2 |
| A4 | `report`, `session --panel` and the dashboard API render `Numbers`; the copies of `panelOut` and `compactorPanel` go | 5 | cmd/downshift, internal/api | A1, A3 |
| A5 | `archtest` rules for the layers, then a golden-log parity test across the surfaces | 3 | `internal/archtest`, testdata | A4 |
| A6 | Delete the unused aggregations; split `telemetry.go` and `session_tree.go` (T11 and T12) | 3 | `internal/telemetry` | A5 |
| M1 | savvy-progress: update the runner-text expectation in `downshift.test.ts`, through `downshift-local.patch` | 2 | `mods/savvy-progress` | — |
| M2 | Panel exposes `saved_total` per spawn and per session, with its scope; the mod renders it and stops adding | 3 | `cmd/downshift/sessionpanel.go`, `mods/savvy-progress` | M1, A4 |
| M3 | Local: update the installed savvy-progress to the version on `main` | 1 | owner's machine | asks first |

Total: 39 points, 14 tasks.

## 3. Waves (cap: 5 agents including the root)

| Wave | In parallel | Why they do not collide |
|---|---|---|
| 1 | K1, A1, A2, M1 | kirocrew adapter, api server, formula package, mod test |
| 2 | K2, A3 | kirocrew child end, accounting package |
| 3 | K3, A4 | kirocrew, surfaces |
| 4 | K4, A5, M2 | shared runner, archtest, panel field and mod |
| 5 | A6 | telemetry deletions and split |

K5 and M3 wait for the owner.

## 4. Not in this plan

- Splitting the vendored `mods/savvy-progress/hooks/register.tsx` (1443 lines): it must stay
  byte-identical to upstream apart from the local patch. A policy note on vendored files is an
  owner decision.
- Saved USD on the surfaces (decision D1 of accounting-v1) waits for A5.
