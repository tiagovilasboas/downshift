# Runtime and evidence task queue

harness-downshift by Tiago de Carvalho Vilas Boas
https://github.com/tiagovilasboas/downshift

Execution snapshot: **2026-10-09**. These are the remaining tasks from the runtime
and evidence review, linked to [beta exit](beta-exit.md). Code shipped, native
data collected, rewrite emitted, and executor model observed are separate states.
Close a task only with its acceptance evidence and a date.

Statuses: pending, running, validation, blocked, done. A blocked task retains its
dependency and next action. Fixtures do not establish a live integration.

## Closure checklist

- [x] GAP-AG-HONOR / P5.5 — executor evidence and QA.
- [ ] GAP-AG-MAPPING — native picker, pool and alias relation.
- [ ] GAP-CURSOR-SOURCE — authenticated native export.
- [x] GAP-CURSOR-AVAILABILITY — native availability consumer; live source still blocked.
- [ ] GAP-CURSOR-AUTOLOAD — actual hook consumes the export.
- [ ] GAP-CURSOR-HONOR / P1.6 — actual child model.
- [ ] GAP-CODEX-HOOK — actual native collector path.
- [ ] GAP-CLAUDE-QUOTA — actual statusline observation.
- [x] GAP-USAGE-COUNT — accurate linked usage and unknown baseline.
- [ ] GAP-USAGE-50 / P3.4 — 50 genuinely linked usage records.
- [x] GAP-COST-NONZERO / P3.5 — token-derived non-zero savings.
- [ ] GAP-CONTROL / P3.6 — comparable real control observations.
- [ ] GAP-PERIOD-REPORT / P3.7 — dated comparison.
- [ ] GAP-USAGE-UPSTREAM / P3.8 — supported child usage sources.
- [ ] GAP-EXTERNAL / P2.3 — independent operator evidence.
- [ ] GAP-README-DATA / P2.4 — dated independent aggregates.
- [ ] GAP-GENERALIZATION / P4 — unseen independent evaluation.
- [ ] GAP-CURSOR-REVALIDATE / P5.3 — dated plan/build recheck.
- [ ] GAP-CODEX-REVALIDATE / P5.4 — dated schema/runtime recheck.
- [x] GAP-REPORT-COUNTS — applied counts across dashboard tabs.
- [x] GAP-FINAL-QA — local integrated gate; remote CI remains separate.
- [ ] GAP-GRAPHIFY / P4.8 — optional work, separately scoped.

## Model availability, quota, and executor

| ID | Status / owner | Acceptance and next action | Dependency / evidence |
|---|---|---|---|
| GAP-AG-HONOR / P5.5 | done 2026-10-09 / root + QA | Correlate the real parent tool call, executed rewrite and exact child; verify model in independent native executor/generation metadata. QA PASS for one observed child on desktop 2.21.1; not a cross-build guarantee. | [Native evidence](evidence/antigravity-executor-ack.md), sanitized JSON and read-only sensor. Streaming rows are multiple observations of one child, not multiple spawns. |
| GAP-AG-MAPPING | blocked / AI-FinOps | Establish an authoritative relation between parent picker IDs, quota groups and `invoke_subagent` aliases. Hold unknown model coverage until then. | Native CLI works: 18 picker models, separate quota groups, no structural membership. [Source evidence](evidence/antigravity-runtime-source.md). Do not infer pool membership from model names. |
| GAP-CURSOR-SOURCE | blocked / Full-stack | Obtain a supported authenticated native export with current balances, dynamic `auto_bucket_models` and complete availability. Verify file and observation time. | Runtime attempted on Cursor 3.24.9: proposal `cursor` is restricted to built-in extensions, including with an explicit proposed-API flag. External bridge fails before transport; no RPC or export. [Diagnostic](evidence/cursor-gap-diagnostic.md). CLI authentication remains separate. No bypass. |
| GAP-CURSOR-AVAILABILITY | done, code only, 2026-10-09 / Full-stack + QA | Connect valid native export availability to routing. Preserve hook precedence and capability floors; provider order is not capability order. | Implemented consumer uses exact IDs, freshness, complete-list evidence and resolver metadata; unknown capabilities hold. Tests validate the code path. Actual source remains blocked under GAP-CURSOR-SOURCE. |
| GAP-CURSOR-AUTOLOAD | blocked / root + QA | Bind the verified export to the actual hook; observe fresh quota affecting a call and invalid/stale quota holding it. | Depends on GAP-CURSOR-SOURCE and GAP-CURSOR-AVAILABILITY. No real export was produced. Installed routing binary is older than this implementation; isolated candidate tests cannot prove the installed route. Preserve private configuration and parent defaults. |
| GAP-CURSOR-HONOR / P1.6 | pending / root + QA | On a supported usage-based plan/build, correlate an emitted model rewrite with the actual child executor model. Record plan/build and mismatch honestly. | Requires the real supported consumer; a balance, parser or export is not executor acknowledgement. |
| GAP-CODEX-HOOK | pending / root + QA | Prove that the actual running hook supplies `transcript_path`, or connect an outside-hook collector. Verify installed binary and a fresh native observation in that path. | [Current replay](evidence/native-quota-routing-2026-10-09.md) proves ingestion and decision, not the live consumer field or executor acknowledgement. |
| GAP-CLAUDE-QUOTA | pending / root + QA | Wire the native statusline bridge while preserving an existing statusline; observe real `rate_limits` in cache and real hook consumption. | Bridge shipped. Inspect before wiring, then validate runtime. `context_window` occupancy is not quota. |

`used_percent` is the consumed fraction of a reported quota window; `100 -
used_percent` gives its relative remainder when valid. It is not dollars and
does not establish a model's membership in a pool. Shared native windows apply
only to their reported scope. Remaining quota does not reserve the next call.

## Usage and cost

| ID | Status / owner | Acceptance and next action | Dependency / evidence |
|---|---|---|---|
| GAP-USAGE-COUNT | done 2026-10-09 / Full-stack + QA | Make `usage_linked` count actual resolved links; retain a separate raw usage count. Unknown baseline must not impersonate a measured baseline. Cover orphan, duplicate, harness/session mismatch and window boundaries. | QA PASS: `usage_records` retains 15 raw records; `usage_linked` counts 5 resolved unique agents. Unknown baselines retain actual cost without fabricating savings. Orphan, duplicate, scope and window regressions passed. |
| GAP-USAGE-50 / P3.4 | pending / dogfood + QA | At least 50 real usage records linked to prior routing decisions, with agent deduplication and dated provenance. | Pre-fix 7-day audit: 15 usage records, **5 resolvably linked unique agents**. Collect genuine completed work; do not manufacture paid calls to meet the count. |
| GAP-COST-NONZERO / P3.5 | done, technical criterion, 2026-10-09 / QA | Verify non-zero savings from native tokens, known baseline/routed models and catalog pricing; preserve assumptions. | Post-fix audit preserved **$0.424866** calculated savings. Five linked records match native transcript models/tokens, including both positive-saving records. [Audit](evidence/billing-gap-audit.md). Billing-dashboard input is not required by this calculation. |
| GAP-CONTROL / P3.6 | pending / dogfood | Collect real `DOWNSHIFT_NO_ROUTE=1` control observations for comparable work and period. | Mechanism shipped; current 7-day audit has **0 baseline records**. Use deliberately scoped real control work; do not change defaults globally. |
| GAP-PERIOD-REPORT / P3.7 | pending / dogfood + docs | Fill the comparison template for a defined period; separate observed tokens, list-price calculation, counterfactual baseline and any invoice reconciliation. | [Template](billing-comparison-template.md). Controlled before/after claims need GAP-CONTROL. Dashboard reconciliation is separate from calculating token-derived savings. |
| GAP-USAGE-UPSTREAM / P3.8 | blocked / Full-stack | Consume native Codex/Cursor child token usage when a supported actual source exposes it and can be linked to a decision. | Track native schemas and add fixtures plus live linkage evidence. Quota percentages are not per-child token usage. |

`internal/telemetry/cost.go` prices provider-reported tokens at catalog list rates;
cached tokens use the input rate, a stated upper-bound assumption. Its baseline
uses the same observed token counts with the known baseline model. It does not
measure a second model's counterfactual token consumption and is not an invoice.
Unknown baselines cannot establish savings. No provider dashboard is needed for
this computation; the separate beta period-report criterion remains tracked.

## Independent evidence and ongoing sensors

| ID | Status / owner | Acceptance and next action | Dependency / evidence |
|---|---|---|---|
| GAP-EXTERNAL / P2.3 | blocked / community | Obtain two non-maintainer 7-day exports, or satisfy the documented independent-source exit alternative, with dates/provenance. | Needs independent operators and consented exports. More runs on one machine do not qualify. No third-party outreach is authorized here. |
| GAP-README-DATA / P2.4 | pending / docs | Update real-session data from verified independent exports with dates and limits. | Depends on GAP-EXTERNAL. Maintainer-only data must remain labeled single-user. |
| GAP-GENERALIZATION / P4 | blocked / curation + QA | Evaluate a frozen classifier on genuinely unseen independently labeled tasks; preserve lineage, rubric, leakage checks and uncertainty. | Historical holdout is a burned regression set. Obtain new data outside tuning exposure; synthetic template variants do not prove generalization. Keep unmeasured routing cost/accuracy explicit. |
| GAP-CURSOR-REVALIDATE / P5.3 | pending / Full-stack + QA | Revalidate advertised plan/build behavior quarterly and after relevant releases; record evidence date and stale state. | Pair contract guia with schema/stale-evidence sensors and real runtime checks. No recurring automation is created by this task queue. |
| GAP-CODEX-REVALIDATE / P5.4 | pending / Full-stack + QA | Revalidate actual `multi_agent_v2` hook schema and spawn behavior quarterly and after relevant changes. | Schema fixtures are sensors; a dated runtime sample is needed before refreshing the matrix. |
| GAP-REPORT-COUNTS | done 2026-10-09 / QA | Confirm All and per-harness tabs count applied shifts consistently and held verdicts do not inflate savings. | Fix shipped: server `applied` flag and dashboard filter. Current API/renderer regressions PASS, including held exclusion and empty harness. Historical browser evidence used the old verdict count; no new browser E2E is claimed. |
| GAP-FINAL-QA | done, local only, 2026-10-09 / QA | Run Go/race/vet, native bridge decoder, precedence/freshness/isolation/tier regressions and adversarial evidence-sensor tests; return PASS/BLOCK with runtime limits. | [Final report](evidence/final-gap-qa.md): PASS for bounded implementation and evidence. Local results do not establish remote CI success or beta graduation. |
| GAP-GRAPHIFY / P4.8 | pending / optional | Keep optional graph command/MCP work separate and measure routing effect before claiming it. | Not a dependency for quota routing, discovery or executor acknowledgement. No implied expansion to Graphify. |

## Execution record and closure

2026-10-09: QA rechecked Go tests, vet and decoder tests; added a renderer test and
quota-precedence regressions. Root collected Antigravity executor evidence;
Cursor diagnostics exposed the missing availability integration. Both fixes and
final evidence review passed the [local QA gate](evidence/final-gap-qa.md). The metadata-only 7-day cost/link audit
was captured near 14:36 UTC (11:36 America/Sao_Paulo); re-run before treating it as
current. No raw logs, account identifiers or transcript paths are stored here.

At 14:47 UTC, the bounded native transcript audit matched all five linked agents
by hash, with streaming-deduplicated token totals and exact native models matching
5/5. Both positive-saving records matched native evidence (2/2). The separate
[billing audit](evidence/billing-gap-audit.md) preserves the pre-fix and post-fix
counts; this satisfies the technical P3.5 criterion with QA PASS. The savings
remain a token-derived list-price counterfactual, not invoice savings. P3.4 still has five genuine links, below fifty; the control count remains
zero and the period comparison remains pending.

The fresh Cursor host attempt reached the built-in-only API restriction before
any request. An external extension and an explicit per-extension API flag did
not produce a quota export. The optional extension was removed and its absence
verified; the temporary window was closed while preserving the user's existing
window. This records an executed investigation and vendor
dependency, not an operational Cursor collector.

Close each task with dated evidence and its acceptance result, then update the
beta row/public matrix. Configuration, parser tests, and success toasts alone do
not close runtime tasks. External prerequisites remain explicit pending work.

Guia inferencial: acceptance, source scope and evidence boundaries above. Sensors
computacionais: schema/tests, expiry guards, link/count audits, CI and prompt-free
telemetry. Sensors inferenciais: QA evidence assessment and independent labeling.
Eixos: behaviour, architecture fitness and maintainability.
