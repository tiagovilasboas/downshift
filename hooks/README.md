# Plugin hooks (layer 3, Claude Code packaging)

`hooks.json` is the Claude Code plugin form of the same four hooks that
`downshift setup` writes into `~/.claude/settings.json`. It holds no logic: each
entry calls the `downshift` binary, which must already be on PATH. Routing,
telemetry and guardrails live in the shared Go layers and serve every harness
([layering rule](../docs/contrib/layering.md)); this file is only how Claude
Code loads them when Downshift is installed as a plugin.

Use one channel, plugin or `setup`, not both: Claude Code runs a plugin hook
and a settings hook separately, so wiring both routes every spawn twice.
`downshift doctor` warns when it finds both.

harness-downshift by Tiago de Carvalho Vilas Boas · https://github.com/tiagovilasboas/downshift
