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

| Kind of work | Model | Effort |
|---|---|---|
| Read-only lookup: find, list, summarise existing files | small (Haiku) | low |
| Mechanical edit: rename, format, boilerplate, one-file change | small (Haiku) | low |
| Review, documentation audit, test writing, a bounded implementation | mid (Sonnet) | medium |
| Design, debugging a hard failure, decisions that span modules | frontier (Opus) | high |

- Built-in agent types have no default model here: `Explore`, `claude-code-guide` and
  `statusline-setup` follow the first row unless you say otherwise; `general-purpose` and
  `Plan` follow the parent unless you say otherwise. Say it on the call.
- Pass the model on the call (`model`). If the harness does not accept a per-call model,
  send a tier hint in the prompt (`[tier=small]`, `[tier=mid]`, `[tier=frontier]`).
- Never send a secret, an auth, payment or data-protection change to a small model
  unless you have read the diff yourself. Downshift no longer blocks it for you.

## 3. Check the result

- `downshift session --why` shows which model each spawn ran on and why.
- `downshift doctor` shows whether the hooks fired and the last decision.
- If a spawn failed, decide the next model yourself. Nothing moves it up or down on its own.

## What Downshift still enforces

- The project `.downshift/policy.json`: rules, profiles and tier floors you declared.
- Unknown model ids and models marked `explicit_only` in the catalog are refused.
- A decision that cannot be made in time or cannot be read keeps the model you asked for.
