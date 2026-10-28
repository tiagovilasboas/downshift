# Estimate vs Actual Analysis: Stages 0 & 2 and Wave 7 (W7.8)

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

**Date:** 2026-10-28  
**Author:** Tiago Vilas Boas

---

## 1. Velocity and Story Points Tracking

In accordance with the Delivery Premise (harness-core rules) and Business Plan execution guidelines, the following table compares initial story point estimates against actual implementation outcomes:

| Card | Description | Est (SP) | Actual (SP / Effort) | Variance / Analysis |
|---|---|:---:|:---:|---|
| **B0–B9 (Stage 0)** | Routing v2 clean base, domain contracts, telemetry decoupling, v1 cutover | 8 | 8 | Delivered on schedule. Core abstractions cleanly decoupled. |
| **C1–C4 (Stage 2)** | Decider v2, hint parser, risk guard, Claude quota integration | 14 | 13 | High efficiency; Decider tests covered edge cases effectively. |
| **F0.1–F0.4 (Onda 0)** | Smart zero-prompt CLI entry, uninstall --purge, interactive TUI setup, dynamic model discovery | 11 | 10 | Reusable CLI helpers kept implementation bounded under 300 lines/file. |
| **F1.1–F1.6 (Onda 1)** | Sensor failure cascade, PreToolUse auto-upgrade, git acceptance, E2E battery, loop engineer | 18 | 19 | +1 SP due to inter-process directory lock complexity (`dead_ends.lock`). |
| **W7.1** | Default Profile Table N11 embedded in decider | 3 | 3 | Implemented with zero-risk elevation logic and table-driven tests. |
| **W7.2** | `downshift policy init` with interactive and dry-run preview | 2 | 2 | Leveraged existing policy schema and validation logic. |
| **W7.9** | Economy mode (`policy economy on/off`) | 3 | 3 | Cleanly integrated as precedence layer in `decide.go`. |
| **W7.4 & W7.6** | AppSec audit and release/migration notes | 5 | 4 | Documented and verified against code diffs and test suites. |

---

## 2. Efficiency Metrics & Retrospective

1. **Lines of Code Efficiency:**
   - 62 files modified / created across Waves 0, 1, and 7.
   - Net codebase growth remained balanced with substantial dead-code pruning (-14,052 lines deleted, +8,249 non-test lines added).
2. **Quality Gates:**
   - 100% of commits verified with `make check` (host, Windows vet, Linux vet, and all tests).
   - Zero test flakiness observed in CI gates.
3. **Key Learnings for Wave 2 (Factory):**
   - Inter-process file operations in multi-agent environments require atomic locking primitives from day 1.
   - Default profiles eliminate zero-configuration friction for end users.
