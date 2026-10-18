<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# N5: one child-end layer for every harness

**Problem (2026-10-13, real log).** Only Claude Code produces child-end and usage
records; cursor, codex, antigravity and kirocrew produce decisions only. So the
report, cost per completed task, rework, the child-end line and the tree's ✓/↻
work only for Claude Code. The end path is Claude-specific
(`claude-code-post-tool-use`, `claude-code-subagent-stop`), which breaks the
layering rule.

**Do.**
- Shared contract: `hookport.ChildEnd{Harness, AgentKey, ToolUseID, Model,
  Status, Usage *TokenUsage, Source}` and one shared runner path that records
  `resolved`, `child_ended` and `usage` for ANY harness (pricing, linking to the
  decision, dedup across PostToolUse/Stop, the child-end line via
  `CanShowChildLine`). The Claude Code commands keep working as aliases.
- Generic CLI entry: `downshift end <harness>` reads the harness's post-spawn or
  stop payload on stdin and hands it to that harness's parser. Adapters only parse
  (`ParseChildEnd(payload) (ChildEnd, bool)`); no recording logic in adapters.
- `HarnessCapabilities` data: `ChildEndEvent` (the hook event name that carries
  the end, empty when none is known) and `UsageSource` (payload, transcript, none).
- Claude Code: move onto the contract with no behaviour change (existing tests
  stay green).
- Codex: parse its PostToolUse for `spawn_agent` (status, child model) and read
  usage from the rollout transcript `token_count` events at `transcript_path`
  (tail read, prompt-free). Use only fields documented or present in local
  fixtures; build fixtures from the shapes in docs/evidence and tests, never from
  live user sessions.
- Cursor, Antigravity, KiroCrew: declare `ChildEndEvent` only where a documented
  event exists; write the parser behind the capability with fixture tests, and
  mark each one "unverified: needs a real payload" in docs/reference/harness-matrix.md.
  Where nothing is documented, leave `ChildEndEvent` empty.
- `downshift report` and `downshift doctor` print, per harness, whether
  completion is measurable (`ChildEndEvent` set and at least one end record seen).
- `downshift setup` wires the end hook for each harness that declares one
  (setup_targets.go data).
- Tests at the shared runner for every harness with a fixture, dedup, linking,
  and "no end event: nothing recorded, no error".
