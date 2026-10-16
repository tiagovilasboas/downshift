<!-- Copyright (c) 2026 Tiago de Carvalho Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# U2: one line when a subagent ends

When a child ends (Claude Code PostToolUse for the Agent/Task tool, or
SubagentStop with usage), return a `systemMessage`:
`↳ downshift: finished on haiku · 14k tokens · $0.01 (opus would be $0.38)`
- Use the records the hooks already build (resolved model, usage, linked
  decision baseline). Missing parts are omitted, never guessed.
- Text comes from the shared formatter (`telemetry.ChildEndLine`); adapters only
  attach it to their wire output. Declare `CanShowChildLine` (or similar) in
  `HarnessCapabilities`; Claude Code true; others false until verified.
- Never blocks, never changes the tool result, never fails the hook; respect an
  opt-out `DOWNSHIFT_QUIET=1` that also silences the existing spawn message.
Tests through the shared runner: line present with full data, partial data,
opt-out, failure path unchanged output.
