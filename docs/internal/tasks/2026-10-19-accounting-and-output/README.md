<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# Handoff 2026-10-19: accounting, tokens-first views and output cutting

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

What happened on 2026-10-19, what is still open and what to do next, in order. Written for the next maintainer or agent.
Rules for every task: small deliveries (one commit, one push, one CI run), tests first, no AI co-author lines, Go copyright
headers, shared layers only (no feature for one harness, [layering.md](../../../contrib/layering.md)).

## Decisions (owner, 2026-10-19)

| Decision | Where it landed |
|---|---|
| Live views show **tokens saved from the requested model**, not dollars: tokens that ran on a cheaper model than requested, work (input+output) and cache counted apart, per agent, per agent type and per session | `telemetry.TokenSavings`; [evidence.md](../../../guides/evidence.md), [spawn-state.md](../../../reference/spawn-state.md) |
| Dollars only in `downshift report`, priced at report time with the current catalog (`prices_as_of`), through an **API lens** (dollars per completed task) and a **plan lens** (tokens); no dollar percentage anywhere | `telemetry.RepriceUsage`, [report.md](../../../guides/report.md) |
| No price guard in `core`. The Codex frontier-under-mid price inversion is the vendor's list price; whether to change the Codex tiers is **undecided** | [value-per-harness.md](../../../reference/value-per-harness.md) anomaly 1 |
| Downshift also cuts tool-output tokens itself, opt-in, default off (`downshift exec`), after AppSec and QA review | PR #259 and the PRs under it (below) |
| Routing becomes an optional feature if it adds less than 10% over RTK alone and shows no quota gain | [benchmark-output-cutting.md](../../../guides/benchmark-output-cutting.md) |

## Landed on main today

| PR | What |
|---|---|
| #241, #244 | Control spawns get end records; custom and plugin agents have a known baseline; credit only for applied rewrites; resumed children priced |
| #245, #254 | Long-brief task focus; short briefs with an explicit `Task:` marker |
| #243, #247 | Per-spawn state (switched/held/observed) for every harness; savvy-progress band |
| #249 | Per-harness value matrix and catalog price audit |
| #250 | Control group keyed on the tool-use id (the copies of one spawn were split across arms) |
| #251 | `tokens_rerouted` and tokens per completed task |
| #252 | Offline simulation gate ([simulation.md](../../../guides/simulation.md)): 10 scenarios, 17 invariants, no model call |
| #253 | Cursor prices from the vendor page; Codex and xAI re-verified |
| #255, #256 | Tokens-first views (tree, panel, statusline, `stats`, `try`, dashboard); report-time pricing |
| #257, #258 | Output-cutting benchmark protocol; adversarial compressor corpus |

## Open pull requests, in merge order

| PR | Base | State | Blocks |
|---|---|---|---|
| #260 | main | Sensor measures Bash output bytes and registers PostToolUse for Bash. Needs `downshift setup` or a plugin update on existing installs | none |
| #262 | main | savvy-progress shipped as 1.2.2 so a cached 1.2.1 with upstream's purple is replaced | none |
| #261 | #259 | Compressor corpus defects D1, D2, D5, D6, D7, D8 fixed (D3, D4 come with #259). Clean-fixture reduction falls from 91% to 58%: the 91% came from dropping package names and coverage | three exec tests need realistic fixtures |
| #263 | #259 | QA adversarial tests for exec, strict-gated | D-A, D-B, D-C |
| #259 | main | `downshift exec`: **do not merge** until the list below is closed | AppSec and QA |

### Must-fix before #259 can merge

From the AppSec review (threat model and checklist in the maintainer notes) and the QA verdict:

1. Rewrite only when the original argv matches a Bash allow rule and no deny or ask rule in any scope; any settings read error means no rewrite; never a `permissionDecision`.
2. `downshift exec` refuses (exit 126, nothing spawned) any argv outside the allowlist.
3. Stop advising an allow rule for `downshift exec -- go test`.
4. Rewritten command uses the single-quoted absolute path of the binary.
5. Child in its own process group; kill the group on stop.
6. Marker line when output is compacted.
7. `DOWNSHIFT_EXEC_DEPTH` guard, shell branch inside the hook deadline, a telemetry canary test, `FuzzDecide`.
8. QA D-A: stdout held until exit flips stdout/stderr order and shows nothing if the wrapper is killed. D-B and D-C are the compressor's D7 and D8 (#261).
9. **Unverified:** whether Claude Code applies `updatedInput` when the hook returns no permission decision. Check once in a live session; if it does not, the only safe decision is `ask`.

## Open findings (not fixed)

| Finding | Evidence | Next |
|---|---|---|
| Codex frontier (`gpt-6-sol` 2/10) is priced below mid (`gpt-5.6-terra` 2/12), so a mid task routed there costs more | [value-per-harness.md](../../../reference/value-per-harness.md) | Owner: retier Codex or accept |
| Normalised units add 1.0 for a pricier downshift (`aggregate.go`), hiding the overspend; the dashboard counts it as a downshift | QA, 2026-10-19 | Count it as pricier in tokens and units |
| Explore tasks under an Opus-tier parent are all held as unconfident | simulation scenario `agent-explore` | Measure on real briefs, then decide |
| 19 of 82 spawns on 2026-10-19 held by `HELD_UNCONFIDENT_DOWNSHIFT` | `stats --export` reason codes | Sample the briefs by hand to see how many were mechanical; recheck after #254 is installed |
| Only Claude Code records child tokens; Cursor, Codex and OpenCode show "not observed" | [value-per-harness.md](../../../reference/value-per-harness.md) | Capture a real payload carrying child tokens per harness |
| Cursor payload marker (`cursor_version`, ...) not captured live | harness-matrix | `downshift probe` in Cursor |
| `TestRunCommand_BackgroundChildDoesNotHang` (`internal/hookctx`) is timing-flaky | failed once in three full runs | Loosen the bound or isolate it |

## Next steps, in order

1. Fullstack closes the #259 list; merge #261 into its branch; AppSec and QA re-review the new head; then merge #259.
2. Merge #260 and #262 (independent). Owner runs `claude plugin marketplace update downshift` and `claude plugin update savvy-progress@downshift`, reinstalls the binary from main and runs `downshift setup`.
3. After a few days of normal use: `downshift report`, `downshift context metrics` (the sensor now has bytes), and the reason-code mix of `stats --export`. This is the first real number after the fixes of the day.
4. Owner decides the Codex tiers (finding 1).
5. Run the pilot of [benchmark-output-cutting.md](../../../guides/benchmark-output-cutting.md) (routing alone, + Headroom, + `downshift exec`, RTK alone), then apply its pre-registered cut criterion.
6. Capture child-token payloads for Cursor, Codex and OpenCode so the tokens-first views cover them.

## Measured so far (do not quote beyond this)

- Routing: about 12% on mechanical delegated work (2026-10-15 A/B, rung D, one operator); about 3% estimated in natural use before the fixes of the day ([real-usage-2026-10-19.md](../../../evidence/real-usage-2026-10-19.md), rung B). The figure after the fixes is unmeasured.
- Output cutting: no real-world number yet; 58% on the clean fixtures of the compressor corpus is a lab figure.
