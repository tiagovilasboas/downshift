---
name: downshift-orchestrator
description: Playbook for the orchestrator (the lead session) when it delegates work to subagents. Use when splitting a task into subagents or choosing the model and effort of a spawn, with Downshift installed.
---

# Orchestrator playbook

You choose the model and the effort of every subagent. Downshift applies your choice,
checks it against the project policy and measures it. It does not pick a model on its own.

## 1. Split the work

1. Restate the task in one sentence. If it reads two ways, ask first.
2. Estimate each part in story points (1, 2, 3, 5, 8, 13). Split any part of 8 or more.
3. Run parts in parallel when they share no file and no pending decision.
4. Do 1–2 point parts yourself: a spawn pays for its own context.

## 2. Choose the model per spawn

Before any delegation, call the Downshift MCP tool `providers` for your harness and read the live list:
you may only pick a model it shows. Each entry gives the tier (small, mid, frontier), the source and the efforts.
A model marked untiered has no tier evidence: name it only on purpose, never through a tier hint.

- `providers`: the models on offer now. Call it first, before the first spawn.
- `apply`: previews what Downshift would apply for the model and effort you chose, under the project policy.
- `playbook`: the working rules to hand a worker.
- `status`: quota headroom and advice; read it before fanning out many workers.

Fallback when the MCP server is absent: run `downshift models available --harness <your harness>`. If that
is missing or prints no list, use the models the harness itself offers and say so.

| Kind of work | Tier | Effort |
|---|---|---|
| Read-only lookup: find, list, summarise existing files | small | low |
| Mechanical edit: rename, format, boilerplate, one-file change | small | low |
| Review, documentation audit, test writing, a bounded implementation | mid | medium |
| Design, debugging a hard failure, decisions that span modules | frontier | high |

- Built-in agent types have no default model here: `Explore`, `claude-code-guide` and
  `statusline-setup` follow the first row's tier unless you say otherwise; `general-purpose` and
  `Plan` follow the parent unless you say otherwise. Say it on the call.
- Pass the model on the call (`model`). If the harness does not accept a per-call model,
  send a tier hint in the prompt (`[tier=small]`, `[tier=mid]`, `[tier=frontier]`).
- Never send a secret, an auth, payment or data-protection change to a small model
  unless you have read the diff yourself. Downshift no longer blocks it for you.

## 3. Check the result

- `downshift session --why` (or the MCP tool `status`) shows which model each spawn ran on and why.
- `downshift doctor` shows whether the hooks fired and the last decision.
- If a spawn failed, decide the next model yourself. Nothing moves it up or down on its own.

## 4. Delegate with an envelope

A worker sees only the task text and its directory, so write the whole brief into each delegated task:

- **Goal:** one sentence, unambiguous.
- **Allowed files:** the exact paths the worker may edit.
- **Verification:** the exact command that must exit 0, run as `downshift compactor -- <command>`.
- **Tier hint:** `[tier=small]` at the start of the prompt for mechanical work: rename, format, boilerplate, a one-file change.

Accept a result only when the verification exits 0 and the diff stays inside the allowed files. You keep the
decomposition and the design. A worker does not split work or choose the architecture.

## What Downshift still enforces

- The project `.downshift/policy.json`: rules, profiles and tier floors you declared.
- Unknown model ids and models marked `explicit_only` in the catalog are refused.
- A decision that cannot be made in time or cannot be read keeps the model you asked for.
