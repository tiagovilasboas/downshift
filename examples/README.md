# Hook payload examples

Use these JSON fixtures to test adapters without a live harness session.

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
| [claude-code-pretooluse-session.json](claude-code-pretooluse-session.json) | Claude Code | Same task with a `session_models` list in the payload, so the hook rewrites (the sample above only allows) |

Set `DOWNSHIFT_SESSION_MODELS` or create `~/.downshift/session-models.json` so
rewrites are allowed. See [docs/session-models.md](../docs/session-models.md).
