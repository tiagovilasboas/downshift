<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# Benchmark protocol: routing, output cutting and a third-party tool

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

Routing alone saved about 12% on mechanical delegated work and an estimated 3% in natural use
([evidence.md](evidence.md), rungs D and B). Output cutting, which shrinks tool output before it reaches the model,
acts on the main session as well. This protocol answers one question before more is built: **does routing still earn
its place once output cutting is in the stack, and how does the stack compare with a third-party output-cutting tool alone?** It extends
[benchmark-protocol.md](benchmark-protocol.md) (tasks, per-run record, privacy, limits) and does not repeat it.

Status: **protocol only. Nothing here has been run.** Every number below is a design parameter or a pre-registered
threshold, not a result.

## Arms

All arms: same harness version, same plan, same repository state, same prompts, same session model, one
`DOWNSHIFT_STATE_DIR` per arm, run order shuffled across arms.

| Arm | Routing | Output cutting | How to set it |
|---|---|---|---|
| 0 (control) | off (`DOWNSHIFT_NO_ROUTE=1`, hooks still record) | none | as arm B of the protocol it extends |
| a | Downshift routing only | none | `DOWNSHIFT_CONTROL_SHARE=0` |
| b | routing | Headroom companion in the request path | arm a plus `headroom wrap` for the harness (Headroom's own command, see [companions.md](companions.md)) |
| c | routing | `downshift compact` wrapper (on by default) | arm a plus the wrapper as documented once released |
| d | off | a third-party output-cutting tool alone, if installed | that tool's own hook setup; `DOWNSHIFT_NO_ROUTE=1` |
| e (optional, isolates routing) | off | `downshift compact` | arm c with `DOWNSHIFT_NO_ROUTE=1` |

Arm d is dropped, and the result says so, if that tool is not available. Arm e is what separates the marginal value of
routing on top of exec (c against e); without it, c against d is the only stack comparison. Confirm each arm's
configuration from the log (`requested_model`, companion record from `downshift doctor`), not from settings files. A
companion's saving is never credited to routing.

## Tasks

Reuse the frozen task set and categories of [benchmark-protocol.md](benchmark-protocol.md#tasks). Output cutting acts
on tool-heavy work, so the set must include tasks with large test, build or log output. Categories: mechanical, medium,
delegation-heavy, and one added here, **output-heavy** (a bounded fix whose loop prints long test or build output).
Complex and security-sensitive tasks stay in the set as safety checks (acceptance only), not as a saving claim. Each
task keeps its hidden acceptance check, validated on base and fix.

### Ecosystem Coverage: Go, Python, TypeScript/Node (vitest, Jest, Playwright, Cypress, Next.js, tsc), PHP (PHPUnit), Rust (cargo test), Dart and Flutter, and the JVM (Gradle, Maven)

The output-heavy category explicitly evaluates suites across the primary agent engineering ecosystems:
- **Go (`go test`)**: Multi-package unit and integration suites with package summaries. A verbose single-package pass is compacted only when test bodies are absent; failure transcripts and test logs remain raw.
- **Python (`pytest`)**: Suites running 50+ tests (e.g. FastAPI/Flask services, data pipelines). Clean runs output dozens of lines of test identifiers, progress percentages, and session summaries, inflating tool output by 1,500–3,500 tokens.
- **TypeScript / JavaScript Unit (`vitest`)**: Fullstack suites running 50+ tests across multiple test files. Clean runs render module trees, execution durations, and checkmark tables, injecting 2,000–4,000 tokens per execution.
- **JavaScript / TypeScript Unit (`jest`)**: Jest prints per-suite results and test summaries. Downshift compacts only when both suite and test summaries show a complete passing run; skipped and todo counts remain visible. Failing or incomplete output stays raw.
- **End-to-End Browser Testing (`cypress run`)**: Cypress emits a completed run summary with totals, spec name, duration, browser details, and optional failure diagnostics. Downshift compacts only complete, internally consistent passing summaries and preserves warnings, errors, or ambiguous output byte-for-byte.
- **Next.js (`next build`)**: A recognized complete build includes compilation phases, static generation, route and shared chunk tables. Downshift keeps route counts by router, shared JavaScript size, and duration while dropping repetitive build phases; warnings, errors, incomplete tables, or unfamiliar output remain raw.
- **PHP (`phpunit`)**: The default dot-progress printer can emit a long row followed by test and assertion totals. Downshift compacts only complete, consistent passing runs without test output; warnings, failures, alternate printers, and ambiguous output remain raw.
- **End-to-End Browser Testing (`playwright`)**: Suites running 10–30 E2E tests across Chromium, Firefox, and WebKit. Clean runs emit hundreds of worker step notifications, browser launch events, test titles, and timing breakdowns, generating **3,000 to 5,000 tokens** per run. Squelching clean runs to `playwright: PASS (N passed across M workers in X.XXs)` (~25 tokens) delivers a ~99% token reduction. Fail-open guarantee: any `failed`, `timed out`, `Error:`, assertion diff, trace attachment, or non-zero exit is preserved 100% verbatim.
- **Systems Engineering (`cargo test`)**: Rust crate builds and test runners. Clean runs output compilation steps, test suite discovery, and lines for dozens of unit/integration tests (`test foo ... ok`), generating **1,000 to 2,500 tokens**. Squelching clean runs to `cargo test: PASS (N passed; 0 failed in X.XXs)` (~20 tokens) achieves 98%+ reduction. Fail-open guarantee: any `FAILED`, compiler warning (`warning:`), compiler error (`error[E...]`), `panicked at`, or non-zero exit is preserved verbatim.
- **Static Typechecking (`tsc --noEmit`)**: Canonical TypeScript typecheck loops in agentic refactorings. When clean, `tsc` exits with code 0 and either an empty string or `Found 0 errors.`. Squelching to `tsc: PASS (0 errors)` (< 5 tokens) provides clear, unambiguous confirmation while preventing tool output ambiguity. Fail-open guarantee: any `error TS\d+:`, diagnostic chain, syntax/semantic error, or non-zero exit is preserved verbatim.
- **Dart and Flutter (`dart test`, `flutter test`)**: A clean run prints progress segments and ends with `All tests passed!`. Downshift summarises only a run with no failure counter and no unknown line; failures, test print output and any stderr stay raw. `dart analyze` and `flutter analyze` are not wrapped.
- **JVM builds (`gradle`, `./gradlew`, `mvn`, `./mvnw`)**: A successful Gradle build collapses its task lines into one count line and keeps `BUILD SUCCESSFUL`. A Maven run keeps its goal lines, Surefire totals and `BUILD SUCCESS`. Any `FAILED`, `[WARNING]`, `[ERROR]` or unknown line stays verbatim.

Tasks in this category simulate realistic agent iterative workflows (e.g., TDD loops, localized bug fixing) where tests and typechecks are executed 2 to 5 times across a multi-turn conversation.

## Metrics

| Metric | Source | Status |
|---|---|---|
| **Cost per accepted task** (primary): cost of every run of the task divided by accepted runs, so a failed run is charged to the task | Claude Code `total_cost_usd` per run (list price) | measured per run by the harness; dollars are list price, not an invoice |
| **Tokens per accepted task, split work and cache**: input+output apart from cache read+write | whole-session usage from the harness; child tokens from usage events | child tokens observed by Downshift; whole-session tokens are harness output, read by hand |
| **Multi-turn Cache Debt**: cumulative cache read tokens incurred across downstream turns ($T_1 \dots T_k$) attributable to tool output from turn $T_0$ | whole-session token progression per turn from harness logs | measured by comparing token delta per turn across arms; isolates output bloat from prompt growth |
| **Quota-window consumption on a subscription plan**: share of the 5-hour and 7-day windows used by the arm's runs | the harness's own usage display before and after a block of runs, same account, no other use | manual and coarse; the only metric that speaks for plans with no per-token bill |
| **Acceptance** | exit code of the hidden check, then the repository suite | manual |
| Retries, failures, rework | as in the protocol it extends | inferred plus manual |
| Latency | wall clock | manual; not a cost claim |

Work and cache are reported apart because cache reads carry the smallest price gap between models and are most of a
session's tokens; one token total would credit routing with a saving it cannot make on that class. Output cutting
shows in fresh-input tokens and, crucially, in **dampening cumulative cache growth across turns**: in a 15-turn session,
squelching a 4,000-token `playwright` pass on turn 2 avoids over 50,000 downstream cache read tokens; squelching a
2,000-token `cargo test` or `pytest` pass on turn 3 avoids over 24,000 cache read tokens; and deterministic `tsc` typechecks
prevent iterative context clutter. Report both single-turn reduction and cumulative session cache savings.

Quota windows are measured in blocks: run one arm's block back to back, read the windows before and after, let the
window reset or use a second identical account, and alternate block order across repeats. The harness rounds window
readings; a difference smaller than one display step counts as no gain.

## Sample size

- Floor: 12 saving-claim tasks (4 mechanical or output-heavy, 4 medium, 4 delegation-heavy) plus 4 safety tasks, 5 runs
  per task per arm, which is 60 saving-claim runs per arm. The earlier A/B used 3 runs per cell and gave a direction
  only.
- A per-category claim needs all 4 tasks of the category at 5 runs (20 runs); fewer is reported as anecdote.
- Intervals: percentile bootstrap, 2,000 resamples, fixed seed, resampling tasks first and runs within a task second.
  The interval is stated whether or not it includes zero.
- Runs that hit the usage limit are rerun, not counted as failures. Runs with a protocol deviation are listed and
  analysed with and without.
- Before the full set, a 2-task, 3-run pilot per arm checks that the arms differ as configured and that the quota
  readings are usable. Pilot runs are not part of the result.

## Pre-registered cut criterion

Fixed before any run, on the saving-claim tasks only, primary metric cost per accepted task:

1. **Acceptance gate.** An arm whose acceptance is more than 5 points below arm 0 or below arm d is disqualified from
   any saving claim.
2. **Routing becomes an optional feature, and stops being the headline,** if both hold:
   - arm c lowers cost per accepted task by less than 10% relative to arm d (that tool alone), judged on the point estimate;
   - the quota-window readings show no gain for c over d (difference within one display step on both the 5-hour and
     7-day windows).
3. If arm d was not run, the same test is applied to arm c against arm e (exec alone): routing adding less than 10% on
   top of output cutting, with no quota gain, makes routing optional.
4. Routing stays a headline feature only if the 95% interval of its marginal saving excludes zero and the point
   estimate is at least 10%.
5. Anything between is reported as inconclusive and the sample is doubled once, not re-cut.

"Optional" means: off by default in `setup`, documented as an advanced lever, no longer described as the main way
Downshift saves tokens. It does not mean removal; the guardrails and evidence tooling keep their value. The threshold
is a decision rule, not a prediction.

## What is measured and what is estimated

Rung per [evidence.md](evidence.md): once run, this protocol produces **rung D evidence** (real tasks, arms against
control, acceptance by hidden tests, one operator) and, if the sample passes, a first input to rung E. It cannot reach
E by itself: `downshift report` defines E on its own rule and needs 20 completed tasks with token usage per group.

| Figure | Class |
|---|---|
| Cost per run from `total_cost_usd` | measured by the harness, at list price |
| Child usage in current events | tokens reported by the harness; current usage events and reports expose no dollar values. Historical schema 2 records may contain catalog-priced USD fields |
| Whole-session tokens and parent coordination | measured by the harness, read by hand; the hook does not see the parent |
| Quota-window consumption | measured, coarse, manual |
| Any ceiling or composition figure in [evidence.md](evidence.md#where-the-saving-can-come-from) | hypothetical until this runs |
| Third-party tool and Headroom figures from their own pages | their claims, not ours; here they are only arms to be measured |

## Limits

Everything in the [limits of the protocol it extends](benchmark-protocol.md#limits-to-state-in-the-result) applies:
list prices not invoices, one operator, direction not proof at small n, structural acceptance checks, Claude Code only.
Added here: quota windows are coarse and depend on account state; output cutting results depend on how much tool
output the task set prints, so the answer is for this task mix and not for natural use; a tool that rewrites output
can change what the agent sees, so acceptance is the guard and any acceptance drop is a finding in itself.

## Output

One page in `docs/evidence/benchmark-output-cutting-<start date>.md`: the arm table as run, task counts per category,
per-arm and per-category results with intervals, the cut-criterion test with its verdict, the quota readings, and the
list of what stayed unmeasured. Aggregates only; no prompts, paths or `events.jsonl`.
