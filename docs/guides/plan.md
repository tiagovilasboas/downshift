<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# Plan

> **Status: in development.** It works, but its interface may change between releases.
>
> Workers (MCP `delegate`): **beta** on Claude (read-only by default, macOS and Linux); **in development** on Codex (no verified real run yet) and on Windows (off by default). Write mode is opt-in.

`downshift plan` checks and tabulates a delegation plan. The lead LLM writes the
parts; Downshift validates them, fills the defaults and prints a markdown
table. Downshift code never calls a model.

## Input

```bash
downshift plan <<'JSON'
{
  "goal": "Add CSV export",
  "parts": [
    {"id": "api", "title": "Export endpoint", "agent": "coder", "points": 3, "files": ["internal/export/handler.go"]},
    {"id": "tests", "title": "Endpoint tests", "agent": "tester", "points": 2, "depends_on": ["api"]},
    {"id": "docs", "title": "Document it", "agent": "documenter", "points": 1, "depends_on": ["api"]}
  ]
}
JSON
```

Fields per part: `id` (unique), `title`, `agent` (a shipped agent), `points`
(1, 2, 3 or 5; 8 and 13 are refused: split the part), `depends_on`, `files`,
`worker` (`native` by default, `claude` or `codex`), and an optional `playbook`
override.

## Output

```
## Plan: Add CSV export

| # | Part | Agent | Tier | Playbook | Pts | Depends on | Wave | Route |
|---|------|-------|------|----------|-----|------------|------|-------|
| api | Export endpoint | coder | mid | implementation | 3 | - | 1 | native |
| tests | Endpoint tests | tester | small | tdd-verification | 2 | api | 2 | native |
| docs | Document it | documenter | mid | docs | 1 | api | 2 | native |

Total: 6 points in 2 waves.
```

Defaults: tier is the agent's `default_tier`; playbook is the agent's
`playbook:` unless overridden; wave is the dependency level (1-based). Route is
`native` for native workers; `claude` and `codex` workers show `direct` until the
gateway switch is enabled, then the gateway name. Parts of one wave that name the same
file produce a warning. `--json` prints the filled plan. Invalid input exits 1
and lists every problem.

## Next steps

After the table, `plan` prints a `## Next steps` section computed from the
filled plan, one block per wave; parts of a wave can run in parallel.

- A native part is an Agent tool call: `subagent_type` `downshift:<agent>` and a
  prompt that names the playbook, the part title, the files and what it depends
  on. The prompt starts with `[tier=small]` only when the tier is small (the
  hint only moves down).
- A `claude` or `codex` part is the arguments of the MCP `delegate` tool
  (`agent`, `mode: read`, `tasks` with the same prompt text), with its tier,
  playbook and route. A `delegate` step appears only for agents the worker mode enables (`downshift doctor` shows it); otherwise the part gets a native spawn step with a warning. Workers route through the
  gateway only when its switch is on (`gateway.json`); otherwise `direct`.

```
## Next steps

Wave 1 (run its parts in parallel):
- api: Agent tool, subagent_type `downshift:coder`, tier mid, prompt: Follow the implementation playbook. Task: Export endpoint. Files: internal/export/handler.go.

Wave 2 (run its parts in parallel):
- tests: Agent tool, subagent_type `downshift:tester`, tier small, prompt: [tier=small] Follow the tdd-verification playbook. Task: Endpoint tests. Depends on: api (already done).
- docs: Agent tool, subagent_type `downshift:documenter`, tier mid, prompt: Follow the docs playbook. Task: Document it. Depends on: api (already done).
```

`--json` adds a `next` array of `{wave, part, kind, subagent_type?, agent?, tier,
playbook, route, prompt}` where `kind` is `spawn` or `delegate`.

Save playbook text with `downshift playbook save <name>` (text on stdin).

## MCP tool

The MCP server exposes the same planner as the read-only tool `plan`. It takes
the stdin JSON of `downshift plan` as its arguments (`goal`, `parts[]` with
`id`, `title`, `agent`, `points`, `depends_on`, `files`, `worker`, `playbook`) and
runs the same code (`plan.Resolve`), including the gateway switch for the route.

The result is the usual envelope (with the `strain` block) whose `data` is:

- `valid`: false when the plan has problems, and then `problems` lists every one
  (a malformed document included); the call does not fail.
- `plan`: the filled plan (`parts` with `tier`, `wave`, `route`, `total_points`,
  `waves`, `warnings`, `next`), present when valid.
- `markdown`: the table and Next steps, as `downshift plan` prints them.

The plan text is answered and dropped: it is never stored. The orchestrator skill
calls `plan` after splitting the work and falls back to `downshift plan` when the
MCP server is absent.

## Worker patterns

Four shipped playbooks describe how to spread one task over cheaper workers: `scout-then-build`, `cascade`, `audit-at-scale` and `best-of-n`. Each states its tier, effort, when to use it, when not to, and what it saves in tokens. List them with `downshift playbook list`; edit them like any other playbook.
