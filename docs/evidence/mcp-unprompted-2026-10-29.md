<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# MCP unprompted and the tier hint, two real Claude Code runs (2026-10-29)

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

Level: **observed twice, one machine.** It answers one question: does the orchestrator call the Downshift MCP
tools by itself, without being told to? It does not, in these runs. It does add the `[tier=small]` hint to the spawn
prompt, and the hook honours it.

## Setup

- Client: Claude Code 2.1.310, `claude -p`, orchestrator `claude-haiku-5-5`, `--permission-mode bypassPermissions`,
  scratch folder only (no repository files touched).
- Server: the user-scope registration (`downshift mcp serve`), binary `02b88696`, with the instructions that lead with
  the rule for when to call `providers`, and `providers` marked for early loading.
- Prompts do not mention Downshift, MCP or tiers.

## Run 1: rename a variable

Prompt: rename `userId` to `accountId` in a small file, "use a subagent (the Agent tool)".

| Item | Result |
|---|---|
| MCP tools called | none |
| Agent call | `downshift:mechanical-worker`, prompt began with `[tier=small]` |
| Hook | event `TRIVIAL`, requested and final model `claude-haiku-5-5` |
| File | correct: `accountId` in both places |

## Run 2: split a small refactor and a test file

Prompt: rename a variable and write a test file, "split the work, delegate what you can".

| Item | Result |
|---|---|
| MCP tools called | none |
| Agent call | `downshift:mechanical-worker`, prompt began with `[tier=small]` |
| Hook | `TRIVIAL` event for the spawn |
| Outcome | the rename was correct; the test file was written and run by the worker |

## What this shows

1. The tier hint reaches the hook from a prompt the orchestrator writes on its own. The hook honours it.
2. The `downshift:mechanical-worker` agent from the plugin is what the orchestrator picks for mechanical work, so
   the hint comes with it. Whether the agent definition or the orchestrator's habit adds the hint was not separated.
3. The MCP tools were not called in either run. The instructions and early loading were in place, so their effect
   on call rate is not shown. A test with a different wording of the instructions is needed before claiming more.

## Not shown

Token savings from these runs (one run each, no control group); whether the hint saves tokens on the parent side;
behaviour on other harnesses.
