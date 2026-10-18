<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# UX: show Downshift working where the developer already looks

Problem: Downshift's activity is visible only in reports and a separately built
monitor (`dsmon`). The developer does not see subagents and routing as they
happen. Fix: native surfaces inside each harness, fed by one shared formatter.

| ID | Task | Surface |
|----|------|---------|
| U1 | Session summary in the Claude Code statusline | statusline bridge |
| U2 | One line when a subagent ends (model, tokens, cost, would-be cost) | hook `systemMessage` |
| U3 | `/downshift` slash command: session tree, why, pause | Claude Code plugin/skill |
| U4 | `downshift watch` replaces the separate `dsmon` build | terminal, every harness |

Rules: shared formatter in `internal/telemetry` (one place for the text);
harness surfaces declared as data in `HarnessCapabilities`; no prompt text is
ever stored (task labels, if shown, are read from the local transcript at
render time only); fail open; output stays short (one line).
