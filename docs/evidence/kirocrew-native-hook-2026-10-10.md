# KiroCrew native policy hook — 2026-10-10

harness-downshift by Tiago de Carvalho Vilas Boas
https://github.com/tiagovilasboas/downshift

Observed locally with Kiro CLI **2.29.0**, the installed KiroCrew macOS app,
and Downshift `dev`. This is policy/block-and-retry evidence, not an in-place
model rewrite or a billing measurement.

## Repair

Registered the absolute executable `scripts/kirocrew-pre-tool-use.sh` in
`~/.kiro/crew/config.json → agent.kiro_hooks.preToolUse`, preserving the Git
push scope guard. After restart, both generated `kirocrew.json` and
`kirocrew-worker.json` retained it. Disabled the old informational Downshift
Hooks-page entry; its invocation counter could not prove enforcement.

Updated only the `kirocrew` allowlist from the installed CLI's actual model
listing: `claude-haiku-4.5`, `claude-sonnet-4.6`, `claude-opus-4.5`.
Installed the [routing guia](../kirocrew-routing-guide.md) as a steering
resource in the active Crew workspace. Kept the vendor application unchanged.

## Native observations

1. Requested MCP `kirocrew-core/spawn_run` with a trivial variable-renaming
   diagnostic, explicit `model=claude-sonnet-4.6`, one turn and no injected
   memory, lessons or project. No file changes were requested.
2. At `14:38:20Z`, the native hook rejected the first call with
   `PreToolHookError`, correlation ID `3f2962b177b9f54a62e3f308793eb9c1`,
   recommending `model=claude-haiku-4.5`. Native UI and the matching
   Downshift `outcome=blocked` event agree.
3. Retried the same MCP tool with that exact model. The hook allowed it.
   KiroCrew queued the child while available memory was below its existing
   threshold. Once eligible, its ordinary one-run approval was accepted;
   no threshold or blanket approval setting was changed.
4. Native child `b6f1e0f1eb3fe4f6` produced a text response. Its native
   `subagents/<id>/state.json` recorded both
   `requested_model=claude-haiku-4.5` and
   **`resolved_model=claude-haiku-4.5`**. The latter is written from
   `provider.served_model` by KiroCrew's subagent manager. The Subagents UI
   also showed that model and streamed output.

The parent default model and its success claims are not the child-model
evidence. Downshift's telemetry normalizes requested IDs through the catalog;
its canonical Sonnet ID is not evidence that an unavailable Sonnet version
was executed. The denied first call did not serve a child model.

Sanitized fields are recorded in [the evidence JSON](kirocrew-native-hook-2026-10-10.json).
It contains no task text, credentials, chat transcript or raw hook payload.

## Checks and limits

- Wrapper syntax and isolated stdin/argument/stderr/exit `0`/`2` checks passed.
- `go test -p 1 ./...` passed: 42 packages, 2 without tests, no failures.
- Three existing adapter smoke fixtures passed in isolated state; they cover
  Claude Code/Cursor, not this native KiroCrew observation.
- Native crew / `use_subagent` without an explicit child model remains
  outside routing coverage and fails open. A single-task MCP `spawn_run`
  with an explicit model is the validated path.
- No automatic rewrite, provider quota guarantee, billing savings or universal
  coverage across vendor versions is claimed. Revalidate after upgrades.

The routing instructions are a **guia inferencial** for **behaviour**; the
native pre-execution policy hook is a **sensor computacional** for that same
concern. Restart persistence and served-model evidence are **sensores
computacionais** for **architecture fitness**. Local routing events provide
continuous observations without proving billing or broader coverage.
