<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# One real spawn through the Claude Code hook (2026-10-30)

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

Level: **observed once, one machine, one spawn.** It shows that the PreToolUse hook installed by `downshift setup`
fires on a real subagent spawn and records its decision. It does not show that an effort value can be applied.

## What was run

- Client: Claude Code in a normal interactive session with the Downshift hook installed, binary built from `main`
  at `a3978e2d`.
- One subagent spawn through the Agent tool: the prompt starts with `[tier=small]`, the call names
  `claude-haiku-5-5`, and the task is "reply with the single word ok, use no tool".
- Before the spawn the event log held 1,379 lines; after it, 1,382.

## What happened

Three events were appended within about a second of the spawn:

| Event | Fields |
|---|---|
| decision | `outcome: allow`, harness `claude-code`, requested model `claude-haiku-5-5`, final model `claude-haiku-5-5`, requested effort `inherit`, final effort `inherit`, verdict `OK`, complexity `TRIVIAL` |
| resolved | requested and final model `claude-haiku-5-5` |
| child ended | requested model `claude-haiku-5-5`, final model `unknown` |

The subagent answered `ok`, and the harness reported 87,645 subagent tokens for the run.

## What it does and does not show

- It shows the hook fires in a real session, the decision reaches the event log, and the spawn ran on the model that
  was requested (the hook kept it: a tier hint only moves a model down, and it was already small).
- It does not show an effort value being applied. The call asked for none and no profile or rule set one, so both
  fields stayed `inherit`. `CanApplyEffort` is declared for Claude Code, but this run did not exercise it.
- It does not show a rewrite: the decision was `allow`, so nothing was changed to measure.
- The child-ended event recorded its final model as `unknown`. That is a gap in what the harness reports at the end
  of a spawn, and it is not explained here.
