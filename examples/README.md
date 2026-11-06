# Hook payload examples

Use these JSON fixtures to test adapters without a live harness session.

```bash
# Run all three fixtures (requires downshift on PATH)
./examples/run-all.sh
```

```bash
# From repo root (Claude Code shape)
downshift claude-code < examples/claude-code-pretooluse-task.json

# Cursor
downshift cursor < examples/cursor-pretooluse-task.json
```

| File | Harness | Notes |
|------|---------|--------|
| [claude-code-pretooluse-task.json](claude-code-pretooluse-task.json) | Claude Code | `Task` tool, trivial rename |
| [cursor-pretooluse-task.json](cursor-pretooluse-task.json) | Cursor | `Task` + `updated_input` shape |
| [claude-code-pretooluse-session.json](claude-code-pretooluse-session.json) | Claude Code | Same task with an explicit `session_models` list in the payload. Without a `[tier=small]` hint or a declared policy the spawn stays on the requested model (owner decision O8: the orchestrator chooses, Downshift applies); with the hint it moves down to `haiku` |

On harnesses without session inference (Cursor, Codex, Antigravity, KiroCrew), set
`DOWNSHIFT_SESSION_MODELS` or create `~/.downshift/session-models.json` so rewrites are allowed. See [docs/reference/session-models.md](../docs/reference/session-models.md).
