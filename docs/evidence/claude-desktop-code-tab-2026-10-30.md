<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# Claude Desktop Code tab: routing, MCP and compaction (2026-10-30)

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

Level: **observed on one machine, from real sessions the owner ran in the Claude Desktop Code tab.** No session was
started for this record; the evidence was read afterwards from the Claude Code transcripts and the Downshift logs.
Card plan: `docs/internal/tasks/2026-10-30-claude-desktop/README.md`.

## How a session was attributed to Claude Desktop

Claude Code writes `"entrypoint":"claude-desktop"` on every line of a transcript started from the Desktop Code tab
(`~/.claude/projects/<project>/<session>.jsonl`). Downshift stores the session id as
`sha256("harness-downshift-session-v1:" + id)` (`internal/telemetry/panel.go`), so a Desktop session's events are
found by hashing its id. Since 2026-10-15, 66 transcripts carry `claude-desktop`, 4 `cli`, 10 `sdk-cli` and 195
`sdk-ts`; 994 of the 1,374 `claude-code` events in that period belong to Desktop sessions.

Session used below: `21bba747-e845-4924-8d4f-0adbcc712ef6`, project `/Users/tiago.boas/Github/downshift`, embedded
Claude Code 2.1.299 to 2.1.309, Downshift `v0.2.0-69-g01b25094`.

## Model routing: works, rewrite honored

| Step | Evidence |
|---|---|
| Spawn | 2026-10-30T21:16Z, Agent tool, `model: "opus"`, prompt `[tier=small] Reply with the single word ok. Use no tool.` |
| Decision | event `47ffb521...`: `rewrite_emitted`, verdict `DOWNSHIFT`, `claude-opus-5-5` to `claude-haiku-5-5`, tier `small`, effort `low` |
| Child | subagent transcript `agent-a5cbfe38cd9b3956f.jsonl`: first assistant message at 21:16:13Z on `claude-haiku-5-5`, answered `ok` |

The child ran on the rewritten model, so the PreToolUse hook in `~/.claude/settings.json` is loaded and honored by
the embedded Claude Code. As in the CLI, the `child_ended` event records the final model as `unknown`.

## MCP: works

The same session called `mcp__downshift__providers` and `mcp__downshift__status` (2026-10-29T14:52Z) and
`mcp__downshift__apply` twice (2026-10-30T21:16Z). Every result came back with `is_error: false` and a JSON body
(`providers` with the model list, `status` with `server: downshift`, `apply` with the requested model). Four other
Desktop sessions also called `providers`.

## Compaction: the rewrite runs; no reduction observed yet

Desktop sessions mostly run commands with pipes or `&&`, which the compactor leaves alone. Of the ten simple
allowlisted commands found in Desktop transcripts, two have a matching exec event seconds later:

| Desktop command | Exec event | Result |
|---|---|---|
| `git status` (2026-10-21T15:06:23Z) | 15:06:26Z, family `git-status`, 308 bytes in and out | went through the compactor, nothing to cut |
| `make install` (2026-10-28T04:31:25Z) | 04:31:30Z, harness `claude-code`, family `make`, passthrough `unrecognized_output` | went through the compactor, passed through |

So the Bash rewrite is applied in the Code tab, but no Desktop command here was compacted with a byte reduction.
Exec events carry no session id, so a compacted event cannot be tied to Desktop by hash; the two rows above are
matched by time and command only.

### Byte reduction with the embedded build (2026-10-30T22:09Z)

The Desktop window exposes no accessibility tree to the automation tool (AX error -25211), so the reduction was
shown by running the Claude Code binary bundled with Claude Desktop
(`~/Library/Application Support/Claude/claude-code/2.1.309/.../claude`, version 2.1.309) headless with `-p`, in
`/Users/tiago.boas/Github/downshift`, with the same `~/.claude` settings and plugin:

| Session | Command the model ran | Exec event |
|---|---|---|
| `9a723105...` | `downshift compactor -- go test -count=1 ./internal/limits/` | 22:08:58Z, `go-test`, 64 bytes, passthrough `small_output` |
| `96a11e6b...` | `downshift compactor -- go test -count=1 -v ./internal/limits/` | 22:09:32Z, `go-test`, `compacted: true`, 2,373 to 218 bytes, exit zero |

Two limits on this row: the transcripts carry `entrypoint: sdk-cli`, not `claude-desktop`, because the binary ran
headless and not inside the Desktop app; and the model wrapped the command itself, following the repository's
instructions, so it shows the compactor working in that build rather than the hook rewriting a plain command. The
hook rewriting a plain command in a real Desktop session is the `make install` row above.

## What this does not show

- A compacted command with a byte reduction inside a session started from the Desktop window itself
  (`entrypoint: claude-desktop`). The reduction above comes from the same build run headless.
- Anything about the Chat and Cowork surfaces: see `downshift-labs/reverse/claude-desktop/findings/F2-chat-mcp.md`
  and `F3-cowork-vm.md`.
- A second machine or another Desktop version.
