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

`opencode/index.ts` is the OpenCode bridge, a different kind of file: an OpenCode
plugin, not a hook list. It is split into `index.ts` plus `models.ts`,
`shell_sessions.ts`, `shell_trust.ts` and `wire.ts` in the same folder.
`hooks/embed.go` embeds them, and `downshift setup` writes the same bytes to
`~/.config/opencode/plugins/downshift/`. OpenCode loads one plugin per directory
under `plugins/` and skips loose files, so the modules are not extra plugins. It holds no
policy either. It calls `downshift opencode` (route, retry and shell answers),
`downshift honor opencode` and `downshift end opencode` (observation batches),
and applies the answers. Every call is fail-open: a missing binary, a timeout or
a bad answer leaves OpenCode's own behaviour in place. Edit the files here, never
the installed copies; `downshift doctor` reports a copy that differs from these.

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift-ai
