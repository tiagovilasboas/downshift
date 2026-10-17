# Compress tool output

Long test, build and log output is paid twice: when it is read, and again whenever the
history is re-read. Two layers exist; pick by harness.

## Headroom (default companion, request path)

A local proxy that compresses tool results before the API and keeps the originals.
Downshift detects it and records it (`downshift context enable headroom`), never installs
or wraps it. The user runs `headroom wrap claude` or `headroom wrap codex` themselves;
Cursor is manual. Privacy: `HEADROOM_BEACON=off` or `DO_NOT_TRACK=1`. Details and the per-harness
table: `docs/guides/companions.md`. Compression numbers come from `headroom savings`, not
from `downshift stats`.

## Native compressor (any harness, shell pipe)

`downshift context compress safe --exit 0` compacts an all-`ok` `go test`, a clean
`git status` and repeated log lines; it preserves every failure, non-`ok` line, search hit,
file list, `git log` and non-zero exit. Without `--exit 0` it preserves the input. Use it in
the command the agent runs, for example:

```bash
go test ./... 2>&1 | downshift context compress safe --exit ${PIPESTATUS[0]}
```

On Claude Code the PostToolUse hook cannot replace a tool result, so the native provider only
observes there (`downshift context metrics`, bytes not tokens). Where a harness has no
output-rewriting hook, the pipe is the only native path; say so instead of promising savings.
