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
| #260 | Sensor measures Bash output bytes and registers PostToolUse for Bash |
| #262 | savvy-progress shipped as 1.2.2 so cached purple 1.2.1 is replaced |
| #259 | `downshift exec`, default off; closed AppSec C1-C13 and QA D1-D8 |

## Open pull requests

| PR | Base | State |
|---|---|---|
| #264 | main | This handoff and the roadmap rows |

### What the exec branch now does (all landed on main in #259)

The AppSec must-fix list and the QA defects, closed and verified:

1. Rewrite only when the user's own Bash allow rule matches the original command and no deny or ask rule does, across managed, user (`CLAUDE_CONFIG_DIR`), project and local settings; any read or parse error means no rewrite; no `permissionDecision` is ever emitted.
2. `downshift exec` refuses anything outside `go test` and `git status` (exit 126, nothing spawned).
3. No advice to allow `downshift exec` in the docs.
4. The rewrite uses the single-quoted, symlink-resolved absolute path of the binary, skipped when not executable.
5. The child runs in its own process group and signals go to the group (a SIGKILL of the wrapper itself still cannot be forwarded).
6. Compacted output ends with `[downshift exec: compacted N to M bytes]`.
7. `DOWNSHIFT_EXEC_DEPTH` guard, the shell branch under the hook deadline, a telemetry canary test and `FuzzDecide`.
8. QA D-A: stdout and stderr keep the program's order; stdout is held at most 2 seconds, and the first stderr byte, 2 MiB or the bound flushes it raw. D-B and D-C close with the compressor fixes of #261.

### Tasks on `downshift exec`

| # | Task | Owner | Done when | Status |
|---|---|---|---|---|
| E1 | AppSec re-review at head `754db877` against the 14-item checklist | AppSec | every item PASS; the maintainer notes carry the verdict | **PASS** (C1-C13 pass) |
| E2 | QA re-run with the real binary (exit codes, ordering, large output, fail-open, prompt-free, strict corpus) | QA | PASS, or defects with failing tests | **PASS** (verified on real binary) |
| E3 | **Live check C14**: does Claude Code apply `updatedInput` when the hook returns no permission decision? Steps are in [harness-matrix.md](../../../reference/harness-matrix.md). If it does not, the only safe decision is `ask` | owner | recorded in the matrix with the Claude Code version | Open |
| E4 | Keep a SIGKILLed wrapper from leaving its child running (a group kill needs a supervisor) | fullstack | a test, or the limit stays documented | Limit documented |
| E5 | Decide whether `git log` joins the allowlist (the compressor keeps it verbatim, so wrapping saves nothing) | owner | decision recorded | Kept off allowlist |
| E6 | Merge #259 | owner | CI green, branch up to date | **MERGED** (in main) |

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

1. E1 and E2 in parallel on head `754db877`, then E3 (owner), then merge #259 (E6).
2. Merge #260 and #262 (independent of the above). Owner runs `claude plugin marketplace update downshift` and `claude plugin update savvy-progress@downshift`, reinstalls the binary from main and runs `downshift setup`.
3. After a few days of normal use: `downshift report`, `downshift context metrics` (the sensor now has bytes) and the reason-code mix of `stats --export`. This is the first real number after the day's fixes.
4. Owner decides the Codex tiers (finding 1 above).
5. Run the pilot of [benchmark-output-cutting.md](../../../guides/benchmark-output-cutting.md) (routing alone, + Headroom, + `downshift exec`, RTK alone), then apply its pre-registered cut criterion.
6. Capture child-token payloads for Cursor, Codex and OpenCode so the tokens-first views cover them.
7. Continue the native output cutting candidate roadmap in [output-cutting-candidates.md](output-cutting-candidates.md): Phase 1 is landed (Playwright, Cargo, TSC, verbose Go tests); Jest, Cypress, Next.js build, and PHPUnit are landed; remaining Phase 2 candidates are Prettier, Python formatting, and Mypy.

## Measured so far (do not quote beyond this)

- Routing: about 12% on mechanical delegated work (2026-10-15 A/B, rung D, one operator); about 3% estimated in natural use before the fixes of the day ([real-usage-2026-10-19.md](../../../evidence/real-usage-2026-10-19.md), rung B). The figure after the fixes is unmeasured.
- Output cutting: no real-world number yet. 58% on the clean fixtures of the compressor corpus is a lab figure, and `exec` rewrites only `go test` and `git status`, so on a repository whose `go test ./...` prints `[no test files]` lines (the compressor leaves those raw) the real cut will be smaller.
