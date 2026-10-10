# KiroCrew subagent routing

harness-downshift by Tiago de Carvalho Vilas Boas
https://github.com/tiagovilasboas/downshift

When delegation is warranted, use the `kirocrew-core` MCP tool `spawn_run`
with a single `task` and an explicit `model` selected from the installed
Kiro CLI's model list. For multiple assignments, issue separate bounded calls
so each task has its own routing decision. Keep the existing concurrency limit.

The native `use_subagent` / crew pipeline does not expose an explicit child
model to this router. Prefer `spawn_run` for model-routed delegation. Never
infer the child's model from the parent's default, a UI title, or a success
message.

If the native PreToolUse hook blocks a spawn, retry it once using the exact
`model=` recommended in the hook's error. For a small model, use a short task
and `include_memory=false`, `include_lessons=false`, `include_project=false`.
Do not replace the blocked call with native crew to evade the routing decision.
If the retry fails, report the failure rather than repeating indefinitely.

Verify the child model in KiroCrew's native subagent record. A blocked routing
event proves enforcement, not execution or credit savings. Missing or unknown
models pass through unchanged; they are outside routing coverage.
