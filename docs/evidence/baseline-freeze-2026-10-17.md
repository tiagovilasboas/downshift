<!-- Copyright (c) 2026 Tiago de Carvalho Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# Baseline freeze manifest (2026-10-17)

Downshift by Tiago de Carvalho Vilas Boas · https://github.com/tiagovilasboas/downshift

What this page answers: which evaluation sets are frozen, by which hash and commit, and which numbers the next run must
reproduce before it may report a delta. It is the readable form of `eval/FREEZE.json` in the private downshift-labs
repository. Aggregates only: no prompts, task text or private paths beyond file names are published here.

**Rule.** A frozen set is never tuned against. New tasks go into a new set with its own freeze entry. A label correction is a
new version of the set, with a new sha256 and a fresh evaluation; frozen labels are never rewritten in place.

## Reference binary

| Item | Value |
|---|---|
| Public commit the baseline was measured from | `2c8f3f4` (`2c8f3f4c6c689b17f7769a06119e1a02fcb308a5`, 2026-10-17) |
| Build rule | `git archive 2c8f3f4 \| tar -x -C <clean dir>; cd <clean dir>; go build -o downshift ./cmd/downshift`, go1.27.1, every `DOWNSHIFT_*` variable unset, throwaway `HOME` |
| Classifier entry | `core.ClassifyWithSemantic`; E5 fuse off, E5 shadow off, default in-process hash semantic boost |
| Binary hash | Not a verification key: the hash changes with the build directory (source paths are embedded without `-trimpath`). The labs run hashed `afa6d585…`, a rebuild of the same commit in this session hashed `709a64b4…` and reproduced every number below. Verify by output equality. |
| Labs commit holding the baseline results | `4c28d70` (results measured at labs `5697a51`); labs HEAD at freeze `40d5d19` (2026-10-17T11:50:49Z, clean tree) |

## Frozen sets

"Frozen at" is the labs commit that first holds the file (the labs repository is a git repository; every set is unchanged
since that commit). Tier accuracy is over all items, confident or not; coverage is the share of confident items; unsafe is a
risky or frontier-expected item whose raw tier is below the expected one, as raw count / confident-only count over the safety
set.

| Set (file) | n | sha256 (short) | Frozen at | Reference numbers | Source |
|---|---:|---|---|---|---|
| Seed regression net (`tasks.json`) | 200 | `e6091376` | labs `bea9f36` 2026-10-11 (byte-identical to the public `benchmark/tasks.json` removed in `be1054ac`, 2026-10-06) | tier accuracy 73.5% (Wilson 67.0–79.1%); coverage 61.0% (122 confident, 95.1% right); unsafe 0 / 0 of 50; FRONTIER→MID 0, FRONTIER→SMALL 0, SMALL→MID 46/100, SMALL→FRONTIER 2/100; gate `--min-tier-accuracy=0.60 --max-frontier-to-mid=0` passes | labs `baseline-2026-10-17.{json,md}` |
| Burned holdout (`holdout.json`) | 300 | `8378233c` | labs `bea9f36` 2026-10-11 | 100.0% (98.7–100%); coverage 96.0% (288 confident, 100% right); unsafe 0 / 0 of 75; gate `--min-tier-accuracy=0.80 --max-frontier-to-mid=0` passes | same |
| Fresh, eval-only (`fresh.json`) | 100 | `6396539f` | labs `bea9f36` 2026-10-11 | 53.0% (43.3–62.5%); coverage 37.0% (37 confident, 94.6% right); unsafe 11 / 0 of 25; FRONTIER→MID 44%, FRONTIER→SMALL 0 | same |
| Heldout2, eval-only (`heldout2.json`) | 120 | `03425eef` | labs `bea9f36` 2026-10-11 (same sha as the 2026-10-03 freeze in the generalization study) | 41.7% (33.2–50.6%); coverage 23.3% (28 confident, 78.6% right); unsafe 18 / 3 of 30; FRONTIER→MID 60%, FRONTIER→SMALL 0 | same |
| Blind vitrine, no longer blind (`blind-vitrine.json`) | 60 | `e1c30e67` | labs `bea9f36` 2026-10-11 | 46.7% (34.6–59.1%); coverage 38.3% (23 confident, 65.2% right); unsafe 7 / 0 of 20; FRONTIER→MID 35%, FRONTIER→SMALL 0 | same |
| M3 multilingual v1 (`m3-v1.json`) | 170 | `d4213e51` (equal to `run-manifest.json`) | labs `c94bcad` 2026-10-16, frozen before any candidate run; labels authored by a model, human adjudication pending | 34.1% (27.4–41.5%); coverage 19.4% (33 confident, 100% right); unsafe 74 / 0 of 77; 90 unsupported-language items; warm p95 3.2 ms. By group: EN 55% at 55% coverage, PT-BR 55% at 30%, ES 25% at 5%, FR 30% at 0%, DE 25% at 0%, JA 25% at 0%, mixed 0% at 0% (30 / 0 of 30 unsafe), delegation 75% at 75% | same; per-group table in the labs `.md` |
| Outcome suite, development (40 task dirs) | 40 (10 per label) | `32854b9a` (sorted dir names + `check_test.go` hashes); all four files per task `11a43975` | labs `bea9f36` 2026-10-11; runs recorded 2026-10-03 | frontier `claude-opus-5.5` 40/40 (20/20 TRIVIAL+SIMPLE); mid `deepseek-v4-flash` 31/40 (TRIVIAL 8, SIMPLE 9, MEDIUM 9, COMPLEX 5; 17/20); small `qwen3-coder-30b` 27/40 (9, 7, 6, 5; 16/20); classifier routing 37/40 at the picked tier (6 small, 22 mid, 12 frontier, recomputed 2026-10-15) | labs `run.json` per tier; `benchmark/report.md` for the routing line |
| Held-out outcome suite, GAP-SUITE (21 task dirs) | 21 (5 TRIVIAL, 4 SIMPLE, 8 MEDIUM, 4 COMPLEX) | `f181fd4e` (same method) | labs `afaf34f`..`9b35d29` 2026-10-11 (tasks), `3bc8757` 2026-10-11 (runs); never frozen before this manifest | 6 of 21 tasks run once: small 2/6, mid 4/6, frontier 6/6; spend USD 0.142 total (small 0.001, mid 0.004, frontier 0.138); the 15 unrun tasks have no reference number | labs `run.json` per tier and the smoke-run note |
| Harness runs, real tasks (`tasks.json`) | 4 | `595424df` | labs `139b19f` 2026-10-15 | base → fix commits (public): `explore-plan-baseline` `fb0dcb7`→`6e807fa`; `antigravity-task-text` `80ee141`→`8515e1c`; `setup-named-group` `7956b19`→`de9768f`; `agent-type-telemetry` `621e29a`→`2cb4dc6`. Per arm: b2 routed 7/8 at $0.65 avg, control 7/8 at $0.77, Codex default 8/8 at about $0.21 (estimate), 0 downshifts applied; b6 routed 8/12 at $0.75, control 9/12 at $0.92, 3 of 34 decisions downshifted | labs `RESULTS-2026-10-15.md`, `RESULTS-b6-2026-10-15.md`; [subagent-routing-ab-2026-10-15.md](subagent-routing-ab-2026-10-15.md) |
| Harness runs, mechanical tasks (`tasks-mechanical.json`) | 4 | `cff1bd3c` | labs `3b4f560` 2026-10-15 | all four at base `a8e389c` (public, beta.12 changelog), no fix commit, 9 validated checks each. b5: routed 12/12 at $0.59 avg, control 12/12 at $0.67; 19 of 56 decisions downshifted; per task routed/control `mech-rename` $0.49/$0.74, `mech-constant` $0.66/$0.77, `mech-doc-comments` $0.59/$0.65, `mech-test-mirror` $0.62/$0.52 | labs `RESULTS-b5-2026-10-15.md` |

Costs in the harness rows are Claude Code `total_cost_usd` at list price, one operator, two or three runs per cell. The labs
HEAD at freeze changed the harness runner (a three-arm protocol) but not the task files, so a run under the new protocol
starts a new results file and does not overwrite these.

## The 69.0% versus 73.5% seed figure

`benchmark/report.md` publishes 69.0% seed tier accuracy (SMALL→MID 49/100, SMALL→FRONTIER 7/100); the
[hardening audit](hardening-audit-2026-10-15.md) reports 73.5% on `f9b531e..429bf42`. Both are correct for their binary, on the
same `tasks.json` (`e6091376`). Binaries rebuilt from `git archive` at each commit, run with `downshift benchmark tasks.json --report`:

| Classifier commits | Seed tier accuracy | Note |
|---|---:|---|
| `be1054ac` (2026-10-06) through `aa3a538` (2026-10-11) | 69.0% | `be1054ac` is the commit that wrote the 69.0% row (git blame still points to it); `5a888e7` (2026-10-10), which report.md cites for the eval-only splits, gives the same |
| `c8f1f8b` (2026-10-11, upshift-overspend fix, semantic cap, PT-BR signals) through `da3cbf4` (2026-10-12) | 66.5% | the figure fell before it rose |
| by `daf92ad` through `a02bec9` (2026-10-12) | 70.5% | the mechanical-edit and command signals (`7962809`) sit in this window |
| `236c057` (2026-10-12, merge `fix/t6-classification`) through `2c8f3f4` (2026-10-17), including `f9b531e..429bf42` | 73.5% | the reference figure in this manifest |

So the discrepancy is a stale row: report.md's "Last updated 2026-10-15" refreshed the adapt and eval-only sections, not the
seed row written on 2026-10-06. The seed row was not edited by this task; it should be refreshed to 73.5% (Wilson 67.0–79.1%,
SMALL→MID 46/100, SMALL→FRONTIER 2/100) with the commit named.

## How to verify

Hashes, from the labs repository root:

```bash
sha256sum eval/benchmark/tasks.json eval/benchmark/holdout.json eval/benchmark/fresh.json \
  eval/benchmark/heldout2.json eval/benchmark/blind-vitrine.json eval/multilang/m3-v1.json \
  tools/harness-runs/tasks.json tools/harness-runs/tasks-mechanical.json
# outcome suites: sorted task directory names, then one sha256sum line per check file
(cd eval/outcomes/testdata && (ls | sort; for d in $(ls | sort); do sha256sum "$d/check_test.go"; done) | sha256sum)
(cd eval/outcomes-holdout/testdata && (ls | sort; for d in $(ls | sort); do sha256sum "$d/check_test.go"; done) | sha256sum)
```

Numbers, with the binary built by the rule above (the command lines are those of the labs README):

```bash
DS=../downshift/downshift

$DS benchmark eval/benchmark/tasks.json --gate --min-tier-accuracy=0.60 --max-frontier-to-mid=0
$DS benchmark eval/benchmark/holdout.json --gate --min-tier-accuracy=0.80 --max-frontier-to-mid=0
$DS benchmark eval/benchmark/fresh.json
$DS benchmark eval/benchmark/heldout2.json
$DS benchmark eval/benchmark/blind-vitrine.json

$DS eval-outcome --tasks eval/outcomes/testdata --runs eval/outcomes/runs --verify
$DS eval-outcome --tasks eval/outcomes-holdout/testdata --runs eval/outcomes-holdout/runs --verify
```

`downshift benchmark --report` prints tier accuracy and the rate columns; confident coverage, unsafe counts and latency come
from the labs script `baseline_eval.py`, which also rebuilds the binary from a clean archive and checks the M3 hash against
`run-manifest.json`. A set whose hash differs from this page is not the frozen set, and its numbers are not comparable.

## Not covered

- The harness-run raw runs (prompts, diffs, transcripts) live outside both repositories and are not hashed.
- The legacy splits carry no risk flag, so their unsafe counts use frontier-labelled items only.
- The 15 unrun held-out outcome tasks have no reference number; the 6 run ones are a single pass each, with the mid tier
  served by four providers in that run.

Related: [benchmark/report.md](../../benchmark/report.md), [hardening-audit-2026-10-15.md](hardening-audit-2026-10-15.md),
[e5-int8-evaluation-2026-10-17.md](e5-int8-evaluation-2026-10-17.md), [guides/evidence.md](../guides/evidence.md).
