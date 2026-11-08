// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

# Roadmap: Context Protection & Reasoning Observability

Downshift by Tiago Vilas Boas  
https://github.com/tiagovilasboas/downshift-ai

**Status:** Planned / Local Roadmap  
**Origin:** Empirical insights from Fabio Akita's *"Competente o Suficiente"* (October 2026) on context-window degradation (*context rot*), reasoning effort sensitivity, and autonomous agent loops.

---

## 1. Problem Statement & Motivation

As documented in empirical studies of 2026 coding agents:
1. **The Context Rot Dilemma:** Long passing test outputs and noisy build logs do not just incur financial token costs; they directly pollute the context window, causing exponential degradation in the agent's attention on earlier instructions, architectural invariants, and business constraints.
2. **Reasoning Effort as the Real Cost Driver:** Across top-tier models, capability in routine coding tasks has converged, but inference cost and latency vary drastically based on `reasoning_effort` (*low*, *medium*, *high*).
3. **The "Stubborn Intern" Failure Mode:** Autonomous agents frequently enter thrashing loops, re-running identical failing commands because repetitive error outputs flood working memory.

Downshift is uniquely positioned at the OS / PreToolUse hook boundary to mitigate all three without an LLM in the loop.

---

## 2. Feature Workstreams

### Workstream 1: "Working Memory Preserved" Metric (1 SP)
* **Goal:** Elevate context protection from a secondary statistic to a primary visibility signal in everyday CLI commands.
* **Current State:** `downshift stats` computes `Token ledger (last 30 days): 331k cut by the compactor (estimate: saved bytes / 4)`. However, `downshift session` only emphasizes routed tokens.
* **Target Behavior:**
  - Display a prominent working-memory protection indicator in `downshift session`:
    ```text
    Context preserved: ~331k tokens of noisy scrollback kept out of working memory
    ```
  - Expose `--json` fields: `tokens_compacted_estimate` and `bytes_compacted_total`.
* **Impacted Packages:**
  - `cmd/downshift/sessionpanel_brief.go`
  - `cmd/downshift/cmd_stats.go`
* **Definition of Done (DoD):** Unit tests in `cmd/downshift/session_test.go` and `cmd/downshift/stats_test.go` verifying calculation and formatting under clean, partial, and empty states.

---

### Workstream 2: Subagent Reasoning Effort Observability (2 SP)
* **Goal:** Make `reasoning_effort` explicit alongside model tier in session inspectability and telemetry.
* **Current State:** Downshift already inspects and mutates `reasoning_effort` in `updatedInput` during hook routing, but session views primarily print the model tier name (`sonnet`, `haiku`, `opus`).
* **Target Behavior:**
  - In `downshift session`, render effort when available:
    ```text
    ├─ subagent 119  haiku [effort: low]   ✓  181k tokens · below session model  moved to haiku
    ├─ subagent 126  opus  [effort: high]  ✓   73k tokens                        left unchanged, already fit
    ```
  - Break down reasoning effort tiers in `downshift report` and `downshift stats --detail`.
* **Impacted Packages:**
  - `internal/routing/contract`
  - `internal/telemetry`
  - `cmd/downshift/sessionpanel.go`
* **Definition of Done (DoD):** Table-driven tests verifying parsing of `reasoning_effort` in session records and graceful fallback when effort is unset.

---

### Workstream 3: CCR Anti-Loop Guard ("Stubborn Intern" Detector) (3 SP)
* **Goal:** Prevent catastrophic context burnout when an agent is stuck in an error loop.
* **Current State:** Command Context Recovery (`internal/ccr`) captures command outputs and computes output hashes/digests for retrieval.
* **Target Behavior:**
  - If the same failing command produces an identical error digest 3 times in a single session:
    - Downshift Compactor emits a structured warning to stderr:
      ```text
      ⚠️ [downshift ccr] Identical failure detected 3x (digest: 4f6b1c38). Context loop detected. Check approach or review previous run with 'downshift retrieve 4f6b1c38'.
      ```
    - Prevents silent re-running loops from consuming remaining context budget.
* **Impacted Packages:**
  - `internal/ccr/`
  - `internal/execwrap/`
* **Definition of Done (DoD):** End-to-end battery test in `internal/execwrap` simulating 3 identical failing runs and asserting warning emission without breaking exit codes.

---

## 3. Delivery Prioritization

| Milestone | Deliverable | Effort | Dependencies | Target |
| :--- | :--- | :---: | :--- | :---: |
| **P1** | Working Memory Preserved in `downshift session` | 1 SP | None (ready to implement) | Next sprint |
| **P2** | Reasoning Effort Observability in `session` & `stats` | 2 SP | P1 | Sprint +1 |
| **P3** | CCR Anti-Loop Guard | 3 SP | P1, CCR digest storage | Sprint +2 |
