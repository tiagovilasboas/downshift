# Domain: Observability & Telemetry

Prompt-free telemetry, child outcome tracking, and cost verification.

You cannot manage what you cannot measure. Downshift incorporates an end-to-end observability pipeline that captures routing decisions, child outcomes, token counts, and code intelligence metrics locally on your machine—without ever recording prompt text, private code, or file paths.

---

## 1. Architectural Principles

1. **Prompt-Free & Privacy First**: Telemetry events (`events.jsonl`) log metadata only: correlation IDs, model names, complexity classes, guardrail triggers, and numeric token counts. Prompt text, file paths, and tool outputs never reach disk or logs.
2. **Every Spawn Routed**: The hook routes every spawn. The only way to leave spawns unrouted is the per-session pause (`downshift session --pause`), an explicit user command. Paused spawns and `outcome: baseline` lines from older versions are kept out of routed figures.
3. **Epistemic Honesty**: Downshift reports tokens, not invoices. A saving is a counterfactual share of observed tokens, and only native provider billing can confirm money.
4. **Child Outcome Observability**: Telemetry observes not only what model was picked, but also whether the child subagent completed, errored, or required rework.
5. **One Tokens-Saved Definition**: `telemetry.TokenSavings` gives the share of child tokens that ran on a lower tier than requested, work and cache apart, per agent type, for `stats`, the session views and `dsmon`; none of them shows a dollar, and `downshift try` names only where one decision's tokens would run. No view shows a dollar: `downshift report` is tokens only (tokens per completed task and rework for routed spawns). The product uses no price.
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
       ├── Paused Event: outcome: baseline, reason: PAUSED
       ├── Usage Event (Claude Code): input_tokens, output_tokens, cache_read_tokens, cache_write_tokens
       ├── Child Ended Event: end_status (completed, error, interrupted, stopped)
       │
       ▼
[Local Query & Analysis Engines]
       ├── `downshift stats`  ──► Aggregates decisions, guardrail holds, tokens saved
       ├── `downshift report` ──► Routed spawns: tokens per completed task and rework per verdict
       ├── `downshift session`──► Interactive tree view of active session subagent hierarchy & tokens
       └── `downshift watch`  ──► Real-time terminal streaming of routing decisions
```

---

## 3. Routed-Only Reporting

Every spawn is routed, so `downshift report` has no unrouted side to compare with:

- **One Routed Row**: spawns, ended, completed, billable tokens per completed task and rework rate.
- **Rework by Routing Verdict**: one row per verdict, with the policy key that turns that verdict off. The report flags nothing and never changes the policy.
- **Excluded Lines**: paused spawns and old `outcome: baseline` lines load but stay out of every routed figure.

---

## 4. Child Endings & Rework Tracking

Subagent failures that trigger immediate re-delegation erase any savings from cheaper models. Downshift tracks the entire child lifecycle:

- **End Status Classifications**: `completed`, `launched` (background task), `interrupted`, `error`, `stopped`, or `unknown`.
- **Rework Detection**: When an errored or interrupted subagent is followed by another spawn in the same session within 10 minutes, the second spawn is flagged as rework.
- **Completion Accounting**: The tokens of failed attempts are attributed to the eventual completed task, penalizing aggressive downshifts that lead to rework.

---

## 5. The Evidence Ladder

Downshift categorizes claims according to a rigorous 5-rung evidence ladder:

| Rung | Standard of Proof | Downshift Status |
|---|---|---|
| **Rung A: List Price Math** | Theoretical savings based on model catalog rate cards (e.g., Haiku vs Opus) | Fully verified (`internal/catalog/catalog_test.go`) |
| **Rung B: Telemetry Estimates** | Counterfactual share of observed child tokens that ran on a lower tier than requested | Implemented (`downshift stats`, tokens only) |
| **Rung C: Controlled Benchmarks** | Fixed task suites evaluated under isolated test conditions | Implemented (`benchmark-protocol.md`) |
| **Rung D: Real-Task A/B** | Side-by-side comparison of real tasks judged by objective test suites | Verified on Claude Code (2026-10-15 A/B trial: -12% on mechanical pieces) |
| **Rung E: Team Tokens per Task** | Longitudinal multi-week routed-versus-control comparison in tokens (effect, tokens per completed task, rework) across multiple operators; no money measured (changed 2026-10-31, owner decision) | Cannot be measured as designed since 2026-11-02: the control arm was removed (see [beta exit criteria](../reference/beta-exit.md)) |

---

## 6. Observability CLI Reference

- `downshift stats [--days=N] [--by-harness]`: Summarize routing decisions, guardrail interventions and tokens saved.
- `downshift stats --export`: Generate a sanitized, privacy-safe JSON export containing aggregate counts and rates for beta graduation tracking or bug reports.
- `downshift report [--days=N] [--json|--text]`: Report the routed spawns: tokens per completed task, rework rate and rework per routing verdict.
- `downshift session --tree`: Render the current session's subagent execution hierarchy, showing assigned models, completion marks, and each subagent's tokens.
- `downshift watch`: Live, low-overhead terminal dashboard displaying subagent spawns and routing choices as they occur.

---

## 7. Deep References

- [Sharing Stats Export Specification](../reference/stats-export.md): Complete schema documentation for sanitized telemetry exports and field definitions.
- [`downshift report` Guide](../guides/report.md): Methodology for routed-spawn figures, rework attribution and rework per routing verdict.
- [Evidence Ladder & Method](../guides/evidence.md): Epistemic framework for validating AI coding agent cost reduction.
- [Session View Guide](../guides/session-view.md): Detailed usage guide for interactive session trees and statusline indicators.
