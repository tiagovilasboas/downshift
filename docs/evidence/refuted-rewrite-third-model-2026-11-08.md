<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# A refuted rewrite that ran on a third model, 2026-11-08

Downshift by Tiago Vilas Boas
https://github.com/tiagovilasboas/downshift

Question: when another `PreToolUse` hook wins the same `Agent` call ([hook
coexistence](hook-coexistence-2026-11-07.md)), does Downshift notice that its model
rewrite was not the one that ran, and do its views say what did run?

## Result

**It notices, when its `PostToolUse` hook is wired (`setup` wires it).** Claude Code
reports the model it launched the child on in `tool_response.resolvedModel`; Downshift's
`PostToolUse` hook writes a `resolved` record linked to the decision, with
`rewrite_honored: false` and the served model in `final_model`. The views that read it
were wrong in one respect, fixed in this change: for a refuted rewrite they assumed the
harness had kept the requested model.

Same two event logs, read by `downshift stats --detail` built from `main` (`af4cdbe`) and
from this change:

| Run | Child was served | `main` | This change |
|---|---|---|---|
| cheap-subagents returned last | sonnet | `Models: opus 1`, `Honor observed 0  unobserved 1` | `Models: sonnet 1`, `Honor observed 0  refuted 1  unobserved 0` |
| Downshift returned last (control) | haiku | `Models: haiku 1`, `Honor observed 1  unobserved 0` | `Models: haiku 1`, `Honor observed 1  refuted 0  unobserved 0` |

The event log of the first run, as written by the hooks:

```text
rewrite_emitted  requested claude-opus-5-5   final claude-haiku-5-5
resolved         requested claude-haiku-5-5  final claude-sonnet-5-5  rewrite_honored false  (linked)
child_ended      requested claude-haiku-5-5  final claude-sonnet-5-5                         (linked)
```

So `main` never claimed haiku for that child, but named opus, a model that did not run,
and filed an observed refutation under "unobserved". An earlier fixture (2026-11-06) that
wired only `PreToolUse` showed `Models: haiku 1`; that came from the missing
`PostToolUse` hook, not from the views.

## Method

- Claude Code 2.1.319, one `claude -p` per run with `env -i`, a throwaway `HOME` and a
  mock Messages API on 127.0.0.1, as in the 2026-11-06 fixture (scripts in the history of
  `docs/evidence/hook-coexistence-2026-11-06/`, commit `b74d723`).
- Hooks: Downshift `PreToolUse` (`Agent|Task`) and `PostToolUse` (`Agent|Task`),
  cheap-subagents 0.1.4 (`cheap_subagents.py`, sha256 `9403ce22…f30f35a`) on `Agent`.
- Call: `Agent`, `general-purpose`, requested `opus`, prompt
  `[model:sonnet] rename the userId variable to userIdentifier` (Downshift answers haiku,
  cheap-subagents sonnet). cheap-subagents was delayed 0.5 s in the first run so that it
  returned last.
- The served model is the `model` on the child's own request to the mock API.

## Not covered

- Token attribution: a refuted rewrite still counts as unchanged for routing tokens
  (`tokenModels`), by design. The saving, if any, belongs to the hook that won.
- `downshift session --tree` was not checked against a third model.
- Only Claude Code; other harnesses report the served model differently or not at all
  ([harness matrix](../reference/harness-matrix.md)).
