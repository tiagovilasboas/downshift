<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# Offline simulation: routing and savings accounting without a model call

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

`internal/simbench` replays synthetic workloads through the real hook path and reads the result back
through the production aggregation. It makes no model call, opens no network connection and writes
only inside a temporary sandbox. It runs inside `go test ./...` (under 2 s; about 15 s under `-race`), so CI fails when an
accounting rule breaks.

```bash
make sim                                              # same as below
go test ./cmd/downshift -run TestSimulation -count=1 -v    # per-scenario tables
```

## What one run does

For each scenario in [`internal/simbench/testdata/sim`](../../internal/simbench/testdata/sim) and each
harness it lists (Claude Code, Cursor, Codex, OpenCode):

1. **Spawn.** It builds the harness's own spawn payload (the PreToolUse, preToolUse or route-request
   shape the adapter tests use) and runs it through the binary's shared runner: `harnessRegistry[h].Hook`,
   which is `runHookAdapter` with that harness's wire. Claude Code reads the parent model from a
   synthetic transcript, so inheritance is exercised too.
2. **Child end.** It sends that harness's end payloads through the shared `runEnd`: Claude Code
   PostToolUse then SubagentStop with inline usage, Cursor subagentStop, Codex PostToolUse plus a
   rollout file, OpenCode child_done. OpenCode's switch chain also goes through `runHonor`.
3. **Usage.** Each child draws input, output, cache-read and cache-write tokens from the scenario's
   profile with a fixed seed. Ids come from the same seed, so the production control-group draw
   (`telemetry.InControlGroup`, about 1 in 10) lands on the same spawns every run. Like the real
   payloads, Claude Code, Cursor and Codex send no `correlation_id`, so the draw is keyed on the
   tool-use id (`controlKey` in the runner); OpenCode's key is its `correlation.child`.
4. **Aggregation.** It reads the sandbox log with `telemetry.ReadEventsFrom` and runs `Summarize`,
   `Aggregate` and `BuildReport` on it. The test also runs `downshift report --json` and
   `downshift stats --export` and requires that they print the same numbers.

The binary implements the `simbench.Driver` port in `cmd/downshift/simbench_test.go`. simbench imports
no adapter (enforced by `internal/archtest`), and the binary does not import simbench.

## Invariants (each is a rule id in `invariants.go`)

| Rule | Promise |
|---|---|
| `hook-exit-zero` | No simulated spawn is blocked |
| `one-decision-per-spawn` | One decision event per spawn, no duplicates, no failure lines |
| `wire-matches-log` | The model written on the wire is the logged final model (via the catalog, since Claude Code writes aliases); an untouched spawn logs final = requested |
| `control-group` | A spawn is control exactly when the production hash says so; control is never rewritten and is logged `baseline` + `CONTROL` |
| `control-linked-to-end` | Every child end links to its decision, control included (the 2026-10-19 defect) |
| `one-usage-per-child` | Exactly one priced usage record per child on a harness with a proven usage source; none on the others |
| `baseline-is-requested-model` | The logged baseline is the model the child would have run on |
| `saved-equals-baseline-minus-actual` | Per event: actual = tokens × ran price, baseline = tokens × requested price, credited saving = baseline − actual only for an applied, honored rewrite, else 0 |
| `summary-totals` | `Summarize` gross, overspend and net equal the per-event sums, and net = gross − overspend |
| `control-excluded-from-rates` | Stats total and report arms count routed and control spawns apart; a control spawn is credited nothing |
| `honored-counted-once` | One honor verdict per applied rewrite; a refused rewrite is refuted and saves nothing |
| `no-saving-on-unknown-baseline` | No baseline cost when the requested model is unknown |
| `no-impossible-percentage` | `estimated_savings` stays in [0, 1); no event saves more than the largest per-token-class list-price gap between the two models (the "~98% cheaper" class) |
| `no-saving-without-rewrite` | A log holding only untouched or refuted spawns shows zero saving, zero overspend and no shift |
| `tokens-match-children` (saved) | The live "tokens saved" figures (`telemetry.TokenSavings`, used by the session tree, panel, statusline and `stats` headline) equal the children's tokens: work and cache saved are the cheaper bucket, totals are every priced child, the scenario's one agent type carries the same numbers, and no share exceeds 100% |
| `tokens-match-children` | `stats` `tokens_rerouted` puts every priced child's tokens in exactly one bucket (cheaper, pricier, unchanged, unattributed) per the definition in [stats-export.md](../reference/stats-export.md); a harness without a usage source reads `unobserved`; `report` tokens per completed task equal the children's tokens over completed tasks |
| `prompt-free` | No file the hook path wrote (state dir, event log, `live.json`, feedback store, project dir) holds task text, a planted canary, a path from a task or a raw harness id |
| `scenario-expectation` | The scenario's own bounds: rewrite share, no downshift on risk tasks, unknown baseline |

`TestSimulation_InvariantsCatchPlantedDefects` plants each defect class into a real simulated log
and requires the matching rule to fire, so the gate is not empty.

## Output

Each scenario prints one table under this label:

> ESTIMATE (evidence rungs A/B): catalog list prices on synthetic tokens, no model call. Not a
> measurement: quality and real token counts need real runs or `downshift report`.

Per harness: spawns, control, routed, rewrites, downshifts, upshifts, holds, kept, honored, unknown
baselines, and three money and token columns:

- **rung B (pipeline)**: what the production aggregation credits from the usage records it wrote. It
  shows `unobserved (no usage source)` for harnesses whose `HarnessCapabilities.UsageSource` is none
  (today Cursor, Codex, OpenCode). Those never record a cost, by design.
- **rung A (what-if)**: the same synthetic tokens priced on the requested and the ran model, for
  every routed spawn the catalog prices. OpenCode's free models are `unpriced`.
- **tokens saved work / cache (routed)**: the share of routed tokens that ran on a cheaper model
  than requested, work (input+output) and cache (read+write) apart, and the tokens on a pricier
  model. Routing moves tokens to another model; it does not remove them. The dollar columns carry
  no percentage. Compare the two: under a Sonnet parent most cache tokens move to Haiku, yet the
  dollar gap is small, because Sonnet and Haiku cost the same on cache reads.

On Claude Code, rung A and rung B must agree. A gap there is an accounting regression.

## What it proves and what it does not

It proves that, for these scenarios, decisions, rewrites, holds, control linkage, honor and the
dollar arithmetic are consistent from the hook payload to `stats` and `report`, on every simulated
harness, without task text on disk.

It does not prove:

- **Quality.** Whether the cheaper child did the task. That needs real runs: the outcome suite, the
  A/B in [evidence.md](evidence.md) rung D, or the [pilot](pilot.md).
- **Real token counts.** The profiles are synthetic. A smaller model may use more or fewer tokens on
  the same task. `downshift report` measures cost and tokens per completed task on real spawns.
- **Real payload shapes.** The shapes are the ones the adapter tests use. Payloads still marked
  unverified in [harness-matrix.md](../reference/harness-matrix.md) stay unverified here.
- **Invoice dollars.** List prices from `internal/catalog/catalog.json`, no plan or discount.
- **Classifier accuracy.** The routing columns show what the current classifier does on these tasks,
  not whether it is right. The labelled benchmarks in rung C do that.

## Adding a scenario

Add a JSON file to `internal/simbench/testdata/sim` (unknown fields are rejected):

```json
{
  "name": "my-scenario",
  "description": "one line",
  "parent_tier": "frontier",
  "spawns": 20,
  "harnesses": ["claude-code", "codex"],
  "agent_type": "general-purpose",
  "tasks": ["rename the variable a to b ({canary})"],
  "usage": {"input": [200, 1500], "output": [300, 2500], "cache_read": [15000, 90000], "cache_write": [0, 8000]},
  "expect": {"*": {"min_rewrite_share": 0.5}}
}
```

- Every task must carry `{canary}`.
- `refuse_rewrites: true` makes the harness ignore every rewrite.
- `explicit_model: true` names the requested model in the call instead of inheriting it.
- Fixture agent types: `sim-inherit` (definition without a model), `sim-pinned` (definition naming
  its model), `sim-undefined` (no definition), and `downshift:<agent>` from the repository's
  [`agents/`](../../agents) directory installed as a plugin.
- Model ids never appear in a scenario: the parent model is the catalog model for `parent_tier`.

## Known gaps the scenarios show today (2026-10-19)

- **Long briefs.** A long brief with a stated mechanical `Task:` is classified COMPLEX and kept on
  the frontier model on every harness (Codex's rewrite only sets the frontier reasoning effort).
  That scenario has no rewrite expectation until the classifier handles it.
- **Explore.** Explore tasks ("find where …") are held as unconfident under an Opus-tier parent.
