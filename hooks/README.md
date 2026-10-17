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

`hooks.json` also lists one **mod** under `modules`: [`cache-tax/`](cache-tax/README.md),
a vendored third-party module (MIT) that guards a cold prompt cache with its
price and keeps the cache warm on request (`/keepwarm`). It is the one
third-party component Downshift bundles (owner decision 2026-10-17), Claude
Code only because only Claude Code loads mods, and it holds no routing logic.
`setup` never writes it: a mod is plugin-only.

Two more vendored mods, [`savvy-progress`](../mods/savvy-progress/README.md) and
[`filetree`](../mods/filetree/README.md), ship as sibling plugins of the Downshift
marketplace under [`mods/`](../mods/README.md), one install each, because a plugin
loads a single hooks module. Neither is part of the `downshift` plugin.

Downshift by Tiago de Carvalho Vilas Boas · https://github.com/tiagovilasboas/downshift
