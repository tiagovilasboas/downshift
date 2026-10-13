<!-- Copyright (c) 2026 Tiago de Carvalho Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# Session view: `downshift session`

What Downshift did in one session, readable at a glance. Works for every
harness; Claude Code also gets the `/downshift` command (see
[install.md](install.md#the-downshift-command-claude-code)).

```
downshift session [--tree|--why|--pause|--resume] [--session=<id>] [--json]
```

## Tree (default)

```
session · 5 subagents · $1.43 (without downshift: $1.31) · over $0.12
├─ subagent 1  sonnet  ✓  $0.22  MEDIUM, confident
├─ subagent 2  haiku   ↻  $0.03  SIMPLE, unsure, held
│  └─ subagent 3  sonnet  ✓  $0.09  MEDIUM, confident
├─ subagent 4  opus    ✓  $1.04  COMPLEX, unsure, risk floor · over $0.52
└─ subagent 5  sonnet  …       -  MEDIUM, confident
```

Columns: subagent, the model it ran on, status, cost, and a short reason.

| Mark | Meaning |
|------|---------|
| `✓` | ended (completed, or a usage record exists) |
| `↻` | redone: it failed and the session spawned again within 10 minutes; the redo is nested under it |
| `✗` | failed, interrupted or blocked, and not redone |
| `…` | running, or no end was observed |

Cost is real when the harness reported usage (`$0.22`), an estimate when only
tokens were seen and the catalog has a price (`~$0.22`), and `-` otherwise.
Estimates never enter the header. A loss is shown as `over $x`, never as a
negative number. The header's "without downshift" is the sum of the baselines
(the model each spawn asked for); a child with no baseline counts at its own
cost. A redo is a hint only: the next spawn may be unrelated work.

`--json` prints the same rows as JSON (`session`, `subagents`, `cost_usd`,
`without_downshift_usd`, `delta_usd`, `rows[]`).

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
  [harness-matrix.md](harness-matrix.md#session-view-u3-2026-10-13).
