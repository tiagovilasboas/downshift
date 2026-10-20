<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# Session view: `downshift session`

What Downshift did in one session, readable at a glance. Works for every
harness; Claude Code also gets `/downshift:status` through the plugin, or `/downshift` with the optional manual command alias (see
[install.md](install.md#the-downshift-command-claude-code)).

```
downshift session [--tree|--why|--pause|--resume] [--session=<id>] [--json]
```

## Tree (default)

```
session · N subagents · N switched, N held · saved N% of work · N% of cache
├─ subagent 1  haiku   ✓  N tokens  MEDIUM, confident
├─ subagent 2  haiku   ↻  N tokens  SIMPLE, unsure, held
│  └─ subagent 3  sonnet  ✓  N tokens  MEDIUM, confident
├─ subagent 4  opus    ✓  N tokens  COMPLEX, unsure, risk floor
└─ subagent 5  sonnet  …  -  MEDIUM, confident
by type: general-purpose: N% work · N% cache
```

Columns: subagent, the model it ran on, status, tokens, and a short reason. The header counts the subagents and the
switched and held decisions, then the session's tokens saved from the requested model. It reads one of
`saved N% of work · N% of cache`, `already on the cheapest model`, `nothing saved yet`, or `tokens not observed` when
no child reported tokens. The `by type:` line gives the same share per agent type. A session paused with `--pause`
says so in the header.

Tokens are the child's input, output and cache tokens as the harness reported them. `-` means no usage record yet.
The view prints no dollar figure: `downshift report` compares tokens per completed task.

| Mark | Meaning |
|------|---------|
| `✓` | ended (completed, or a usage record exists) |
| `↻` | redone: it failed and the session spawned again within 10 minutes; the redo is nested under it |
| `✗` | failed, interrupted or blocked, and not redone |
| `…` | running: spawned less than 2 hours ago, no end or usage record yet |
| `?` | unknown end: still no end, usage or error record 2 hours after the spawn (the harness never said it finished) |

A redo is a hint only: the next spawn may be unrelated work.

`--json` prints the same rows as JSON: `session`, `subagents`, `tokens` (with `by_agent_type`), `paused`,
`switched`, `held` and `rows[]`. Each row carries `index`, `depth`, `label`, `model`, `status`, `tokens`,
`tokens_label`, `state`, `reason`, `reason_codes`, `from_model` and `time`.

## `--why`

For the latest decision of the session: what was asked for, what ran, verdict
and tier, every reason code in plain words, and the guardrail that held it
(`HELD_*`, `CREDIT_HELD`, `PAUSED`). Events written before reason codes existed
say so instead of guessing.

## `--pause` and `--resume`

`--pause` writes a marker for the session under the state dir
(`paused/<hashed session id>`); `--resume` removes it. While the marker exists,
the shared hook runner routes like the control group (`DOWNSHIFT_NO_ROUTE=1`):
it classifies and logs, never rewrites, and records the decision with outcome
`baseline` and reason `PAUSED`. The check is one `stat`, fails open and never
blocks a spawn. A marker expires after 12 hours.

## Live: `downshift watch`

```
downshift watch [--session=<id>] [--interval=1s]
```

The same tree, redrawn every interval (minimum 200 ms) from a bounded tail of
the event log, with plain ANSI and no dependencies. Without `--session` it
follows the newest session, so it keeps working when a new session starts; with
an empty log it shows `no subagents yet` and waits. Ctrl-C quits. `--once` draws
a single frame without clearing the screen. It works for every harness and
writes nothing. It replaces the separate `dsmon` build for watching a session
(`dsmon` stays buildable).

## Which session

1. `--session=<id>`: the harness's own id, a full hashed id, or a unique prefix
   (8+ hex characters) of a hashed id as shown by `--json`.
2. The environment variable a harness declares in
   `HarnessCapabilities.SessionEnv` (Claude Code: `CLAUDE_CODE_SESSION_ID`).
3. The most recent session in the event log.

## Privacy and limits

- Rows are labelled `subagent N`. Task text and task labels are never stored
  and never shown; the event log holds only routing metadata.
- It reads the last 2 MB of the event log, so a very long session shows only
  its recent subagents. It writes nothing except the pause marker.
- A session needs an id in the hook payload. Claude Code and Codex send one.
  Cursor, Antigravity and KiroCrew do not, so their spawns carry no session id:
  the view cannot group them and `--pause` cannot target them. See
  [harness-matrix.md](../reference/harness-matrix.md#session-view-u3-2026-10-13).
