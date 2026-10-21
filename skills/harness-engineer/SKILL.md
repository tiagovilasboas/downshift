---
name: harness-engineer
description: Plan and split multi-step agent work with Downshift. Use when the user names Downshift and asks how to split or parallelize subagent tasks, or why delegated work is not getting a smaller model.
---

# Harness engineer (basic)

1. Restate the request in one sentence. If it can be read two ways, ask first.
2. Estimate each part in story points (1, 2, 3, 5, 8, 13). Split any part of 8 or more.
3. Run parts in parallel when they share no file and no pending decision.
4. Do 1–2 point parts yourself; a spawn pays for its own context.
5. Do not pin a model on mechanical subagents: Downshift can only right-size a spawn it may rewrite.
6. Check the result: `downshift session --why` (why each spawn got its model) and `downshift report` (tokens; Compactor bytes stay separate).
