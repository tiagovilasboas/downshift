# Domain: Observability & Telemetry

Prompt-free telemetry, empirical control groups, child outcome tracking, and cost verification.

You cannot manage what you cannot measure. Downshift incorporates an end-to-end observability pipeline that captures routing decisions, child outcomes, token counts, and code intelligence metrics locally on your machine—without ever recording prompt text, private code, or file paths.

---

## 1. Architectural Principles

1. **Prompt-Free & Privacy First**: Telemetry events (`events.jsonl`) log metadata only: correlation IDs, model names, complexity classes, guardrail triggers, and numeric token counts. Prompt text, file paths, and tool outputs never reach disk or logs.
2. **Empirical Control Group**: Approximately 1 in 10 spawns (`DOWNSHIFT_CONTROL_SHARE=0.10`) is left unrouted as an automated control group, enabling scientific, statistically valid comparisons between routed and unrouted workloads.
3. **Epistemic Honesty**: Downshift clearly distinguishes **catalog-priced estimates** from **provider invoices**. Savings figures represent counterfactual catalog benchmarks unless corroborated by native provider billing.
4. **Child Outcome Observability**: Telemetry observes not only what model was picked, but also whether the child subagent completed, errored, or required rework.
5. **One Tokens-Saved Definition**: `telemetry.TokenSavings` gives the share of child tokens that ran on a cheaper model than requested, work and cache apart, per agent type, for `stats`, the session views, `dsmon` and the dashboard; none of them shows a dollar, and `downshift try` names only where one decision's tokens would run. Dollars appear only in `downshift report`, priced at report time from the embedded catalog; its `prices_as_of` date is refreshed by the weekly `sync-prices` workflow ([prices policy](../reference/prices.md)).
6. **No Per-Spawn fsync**: Event appends are plain `write(2)` calls. A process crash loses nothing; a power loss can drop the last events. The logs are statistics ([measurement](../evidence/perf-2026-10-19.md)).
7. **Zero External Telemetry Sinks**: Events remain strictly on your local machine under `~/.downshift/events.jsonl` (files `0600`, directory `0700`). Rotated backups (100 MB default) are not read by `stats`, which prints a note. No telemetry is transmitted to remote analytics servers.

---

## 2. Event Lifecycle & Local Storage

```
[Routing / Hook Event]
       │
       ▼
[Local Event Logger] ──(writes to `.downshift/events.jsonl`)
       ├── Decision Event: timestamp, correlation_id, harness, requested_model, routed_model, rule_tags
       ├── Control Event: outcome: baseline, reason: CONTROL
       ├── Usage Event (Claude Code): input_tokens, output_tokens, cache_read_tokens, list_price_usd
       ├── Child Ended Event: end_status (completed, error, interrupted, stopped)
       └── DCI Event: prompts_evaluated, injections, abstentions, duration_ms, bytes_emitted
       │
       ▼
[Local Query & Analysis Engines]
       ├── `downshift stats`  ──► Aggregates decisions, guardrail holds, estimated net savings, DCI metrics
       ├── `downshift report` ──► Statistical comparison of routed vs control arms (cost per completed task)
       ├── `downshift session`──► Interactive tree view of active session subagent hierarchy & costs
       └── `downshift watch`  ──► Real-time terminal streaming of routing decisions
```

---

## 3. The Built-in Control Group

To evaluate whether routing actually saves money without degrading completion rates, Downshift implements a built-in control arm:

- **Deterministic Hash Assignment**: Spawns are deterministically assigned based on the correlation ID. Approximately 10% of spawns run on the unrouted baseline model requested by the harness.
- **Fail-Open Baseline Execution**: Control spawns are classified and logged identically to routed spawns, but the hook returns the fail-open response, allowing the subagent to run on its requested model.
- **Scientific A/B Comparison**: `downshift report` joins decision logs, token usage, and completion records to calculate cost per completed task and rework rates across both groups with a 95% bootstrap confidence interval.

---

## 4. Child Endings & Rework Tracking

Subagent failures that trigger immediate re-delegation erase any savings from cheaper models. Downshift tracks the entire child lifecycle:

- **End Status Classifications**: `completed`, `launched` (background task), `interrupted`, `error`, `stopped`, or `unknown`.
- **Rework Detection**: When an errored or interrupted subagent is followed by another spawn in the same session within 10 minutes, the second spawn is flagged as rework.
- **Completion Accounting**: The cost of failed attempts is attributed to the eventual completed task, penalizing aggressive downshifts that lead to rework.

---

## 5. The Evidence Ladder

Downshift categorizes claims according to a rigorous 5-rung evidence ladder:

| Rung | Standard of Proof | Downshift Status |
|---|---|---|
| **Rung A: List Price Math** | Theoretical savings based on model catalog rate cards (e.g., Haiku vs Opus) | Fully verified (`internal/catalog/catalog_test.go`) |
| **Rung B: Telemetry Estimates** | Counterfactual savings computed from observed child tokens against list prices | Implemented (`downshift stats`) |
| **Rung C: Controlled Benchmarks** | Fixed task suites evaluated under isolated test conditions | Implemented (`benchmark-protocol.md`) |
| **Rung D: Real-Task A/B** | Side-by-side comparison of real tasks judged by objective test suites | Verified on Claude Code (2026-10-15 A/B trial: -12% on mechanical pieces) |
| **Rung E: Team Cost per Task** | Longitudinal multi-week provider invoice reductions across multiple operators | In progress (tracked under [beta exit criteria](../reference/beta-exit.md)) |

---

## 6. Observability CLI Reference

- `downshift stats [--days=N] [--dci] [--by-harness]`: Summarize routing decisions, guardrail interventions, estimated dollars saved, and DCI latency/injection metrics.
- `downshift stats --export`: Generate a sanitized, privacy-safe JSON export containing aggregate counts and rates for beta graduation tracking or bug reports.
- `downshift report [--days=N] [--json]`: Run an empirical comparison between the routed group and the control group, measuring cost per completed task and rework rate.
- `downshift session --tree`: Render the current session's subagent execution hierarchy, showing assigned models, completion marks, and individual subagent costs.
- `downshift watch`: Live, low-overhead terminal dashboard displaying subagent spawns and routing choices as they occur.

---

## 7. Deep References

- [Sharing Stats Export Specification](../reference/stats-export.md): Complete schema documentation for sanitized telemetry exports and field definitions.
- [`downshift report` Guide](../guides/report.md): Methodology for routed versus control group calculations, rework attribution, and bootstrap intervals.
- [Evidence Ladder & Method](../guides/evidence.md): Epistemic framework for validating AI coding agent cost reduction.
- [Session View Guide](../guides/session-view.md): Detailed usage guide for interactive session trees and statusline indicators.
