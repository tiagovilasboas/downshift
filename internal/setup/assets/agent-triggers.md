# Downshift agent triggers

Downshift by Tiago Vilas Boas
https://github.com/tiagovilasboas/downshift

These phrases in the user's message are commands to dispatch subagents now,
in this same turn:

- `downshift full power`, `downshift full-power`, `full-power`,
  `potência total`, `potencia-total`, `força total`, `forca-total`,
  `downshift full force`, `full-force`: the Downshift squad the task needs, up
  to 5 agents at once (not always all 12).
- `downshift shift <squad>`: one squad. The squads are `security`, `quality`,
  `core` and `leadership`.

When one appears:

1. Take the task from the message, or from the last few messages when the
   phrase stands alone. Ask one question only if there is no task at all.
2. Do not explore alone. Delegate the exploration: a researcher or architect
   agent maps the code while the others start.
3. Split the work into independent slices that touch disjoint files. Size the
   squad to the task: never more than 5 agents at once; a small or indivisible
   task gets one agent.
4. Before spawning, call the downshift MCP `providers` tool and read its
   `strain` block. If headroom is low or a provider is rate-limited, shrink the
   squad or ask the user instead of spawning everyone.
5. Spawn the agents in one message so they run in parallel. Use the Downshift
   agents (`downshift:coder`, `downshift:researcher`, `downshift:security`,
   `downshift:reviewer`, `downshift:documenter`, ...) or this harness's own
   subagent tool.
6. Integrate and verify their results before you call the task done.

The full playbook is the `downshift-orchestrator` skill.
