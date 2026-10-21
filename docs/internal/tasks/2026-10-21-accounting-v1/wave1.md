<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# Wave 1: briefing (S0, S1, S2, T1)

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

**Status: approved by the owner on 2026-10-21, NOT STARTED.** The owner asked for the plan to be
documented only; nothing in this file has been run, branched or opened. Start only when the owner
says so. Roadmap: [`README.md`](README.md). Cards: [`tasks.md`](tasks.md).

## 1. Owner's execution premises (verbatim intent, in force for every wave)

1. **Spikes first.** Validate hypotheses on real evidence before any definitive implementation.
2. **Parallel.** Independent cards run in parallel, respecting dependencies.
3. **MVP.** Small, isolated changes; no overengineering.
4. **Compatibility.** Preserve today's Downshift behaviour; no regressions.
5. **Tests.** Every implementation has automated tests and reproducible evidence.
6. **Accounting.** Percent is the primary saving metric, API-equivalent USD the secondary, both
   derived from the same source (README section 3.1).
7. **Git.** Small commits per task; merge only with green CI.

Goal of the whole effort: raise the precision of the accounting without adding needless complexity.

Standing rules from this session that also apply: no merge without the owner's review for PRs that
change behaviour (#428, #429 pattern); never `--admin`; an agent that finds a blocker stops and
reports; read-only on `~/.downshift`; aggregates only, no prompts, task text or paths in any note.

## 2. Decisions assumed for wave 1

D1 to D6 are **confirmed by the owner (2026-10-21)**: D1 percent primary, API-equivalent USD
secondary; D2 compaction priced once as input; D3 upshifts count as negative saving; D4 inferred
baselines included and labelled; D5 control calibration printed at 20 tasks per group; D6 `stats`
becomes an alias of `report --days`. The spikes below are what can overturn D2 and D4; if they do,
the owner decides again before wave 2.

## 3. Cards

| Card | Type | Pts | Output | Touches product code |
|---|---|--:|---|---|
| S0 formula vs battery | spike | 3 | evidence note + throwaway script in downshift-labs | no |
| S1 compaction persistence | spike | 2 | evidence note | no |
| S2 price coverage | spike | 2 | evidence note | no |
| T1 event fields | implementation | 2 | one PR | yes (`execwrap`, `cmd/downshift`) |

Collision matrix: S0, S1, S2 read only (the private battery runs, `~/.downshift`, the catalog) and
write one note each under `docs/evidence/`; T1 edits `internal/execwrap/metrics.go`,
`cmd/downshift/exec.go`, `session.go`, the hook runner and `docs/reference/spawn-state.md`. No file
is shared. Active agents: four plus the root (cap 5).

### S0. Validate the formula on the battery (3)

- **Hypothesis:** `Saved = C_baseline − C_actual` computed from tokens and requested models predicts
  the measured difference between arms within an acceptable bias.
- **Inputs (private, read-only):** `~/.downshift-eval/runs/four-arm-part1-2026-10-21/**/result.json`
  and `stream.jsonl` (`modelUsage` per model, `total_cost_usd`, `routing`, arm), 46 runs; the
  embedded catalog at origin/main (`prices_as_of` 2026-10-19).
- **Method:** a script in downshift-labs (not in this repo) that, per run, rebuilds `ChildRun`s
  (tokens per model; requested model from the arm: Opus for A, the parent's session model for
  B/C/D), prices them with the catalog, and computes `C_actual` and `C_baseline`. Check
  `C_actual` against Claude Code's own `costUSD` per model (ground truth, tolerance reported). Then
  compare the formula's `Saved` for C and D against the measured B−C and B−D (cost and tokens).
  Report bias per task and per arm with a bootstrap interval.
- **Evidence output:** `docs/evidence/accounting-formula-validation-2026-10-21.md`: method, table
  (estimate vs measured, per task and arm), bias with interval, tasks where it breaks, and the
  assumptions that must be printed next to `Saved` (README 1.1 rule 6).
- **Done when:** the note ends with a go / no-go for `net_counterfactual_cost_v1` and the maximum
  bias the owner can expect; the script is reproducible (`python3 script.py <batch dir>`).
- **Blockers to report, not work around:** runs missing `modelUsage`; prices missing for a model id
  in the runs; an arm whose requested model cannot be known.

### S1. How long does compacted output stay in context? (2)

- **Hypothesis:** pricing compaction once as input (v1) understates the saving enough to matter, or
  does not.
- **Inputs (private, read-only):** the same 46 `stream.jsonl` transcripts.
- **Method:** for each Bash tool result inside a subagent, count the later turns of that agent
  (the result stays in context); estimate bytes/4 tokens; price v1 (once as input at the requested
  model) and v2 (one cache write plus one cache read per remaining turn). Report the distribution of
  remaining turns and the v1/v2 ratio. Use only runs of arm D plus a sample of arm C for contrast.
- **Evidence output:** `docs/evidence/compaction-persistence-2026-10-21.md`.
- **Done when:** a recommendation for D2 with the measured ratio; if v2 differs from v1 by less than
  the S0 bias, v1 stays and the note says why.
- **Blocker to report:** arm D had almost no wrapped commands in part 1 (the Compactor was
  measured as not measurable there); if fewer than 30 compacted-eligible results exist, say so and
  use the corpus fixtures with a stated assumption instead of inventing turns.

### S2. Price coverage of the log (2)

- **Hypothesis:** every model id seen in the events has a complete price; where not, the formula's
  `quality` vocabulary covers the gap.
- **Inputs (read-only):** `~/.downshift/events.jsonl` and its `.bak-*` files (model ids only, no
  prompts), `internal/catalog/catalog.json` at origin/main.
- **Method:** list every `requested_model`, `final_model` and `observed_model` by harness; for each,
  record which of input, output, cache read, cache write the catalog has, whether a tier threshold
  applies (haiku-5-5 above 100k, grok above 200k), and whether the id is a gateway slug without a
  price (audit #423). Count the share of children and of tokens in each class.
- **Evidence output:** `docs/evidence/price-coverage-2026-10-21.md` with the table and the proposed
  `quality` values (`exact`, `upper_bound`, `inferred`, `unpriced`).
- **Done when:** the note states which ids would be excluded as `unpriced`, which would be
  `upper_bound`, and the share of today's tokens each affects.

### T1. Event fields `harness`, `entry`, baseline (2)

- **Why:** direct Compactor calls cannot be attributed to a harness today, and the #410 fallback
  credits them to whichever session id the environment holds (audit A22, S1).
- **Change (MVP):** exec events gain `harness` (from the adapter that wrapped; `unknown` when none)
  and `entry` (`hook` | `direct`). Direct calls stop reading `CLAUDE_CODE_SESSION_ID`: a direct call
  carries no session unless `--session` is given. Old rows read as `entry: unknown`. No new
  subcommand, no schema break (fields are additive and `omitempty`-safe for old readers).
- **Compatibility:** `downshift compactor -- <cmd>` output, exit code, kill switches and existing
  `--session` behaviour do not change; the session fallback removal is the only behaviour change and
  is documented in CHANGELOG.
- **Tests:** a hook-wrapped run records `entry: hook` and the harness; a direct run records
  `entry: direct` and no session even with `CLAUDE_CODE_SESSION_ID` set; an old row without the
  fields is read as `unknown`; `SummarizeSession` still matches only the given hash.
  `go test ./...`, `./examples/run-all.sh`, tests isolate `HOME` and `DOWNSHIFT_STATE_DIR`.
- **Git:** one branch `feat/exec-event-entry-harness`, small commits (fields; direct-call session;
  docs and CHANGELOG), one PR. Merge only with green CI; because it changes attribution behaviour it
  waits for the owner's review, like #428 and #429.
- **Files:** `internal/execwrap/metrics.go`, `cmd/downshift/exec.go`, `cmd/downshift/session.go`,
  the hook runner call site, `docs/reference/spawn-state.md`, `CHANGELOG.md`.

## 4. Stop conditions

- Any spike that finds the formula biased beyond what S0 calls acceptable stops wave 2 and returns
  to the owner with the table.
- Any card that needs a file outside its list stops and reports.
- A red CI on T1 is fixed on its branch; no retry loops, no `--admin`.
- A hook, a test or a tool that writes into `~/.downshift` stops the wave (see curation 5.6).

## 5. Report at the end of wave 1 (template; nothing filled yet)

1. **Spike results:** S0 (bias with interval, go / no-go), S1 (v1 vs v2 ratio, D2), S2 (unpriced and
   upper-bound shares).
2. **Technical decisions:** what changed in D2 to D6 and why, with the evidence line that decided it.
3. **Files changed:** the three notes, the labs script path, and the T1 diff stat.
4. **Tests run:** command and result for T1; how each spike can be reproduced.
5. **Blockers and risks:** anything that could not be measured and what it blocks in wave 2.
6. **Next:** the exact wave-2 cards (T0, T2) and whether the owner's approval of the spec is needed
   first.

## 6. Not started: checklist before "go"

- [ ] Owner says "go" for wave 1 (the owner will say it; do not start before).
- [x] Owner confirmed D2 to D6 as written (section 2).
- [ ] #428 and #429 reviewed (they are independent of this wave).
