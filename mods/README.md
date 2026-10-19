# Mods

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

The Downshift marketplace offers the `downshift` plugin plus sibling mods, each
a vendored third-party Claude Code mod (MIT, pinned in [NOTICE](../NOTICE)), except `downshift-dock`, which is original Downshift code:

| Plugin | Directory | Upstream |
|---|---|---|
| `savvy-progress@downshift` | [`savvy-progress/`](savvy-progress/README.md) | johnnyvizz, v1.2.1 |
| `filetree@downshift` | [`filetree/`](filetree/README.md) | Kurt Buhler, v0.2.26 |
| `downshift-dock@downshift` | [`downshift-dock/`](downshift-dock/README.md) | Downshift (original, Apache-2.0, not vendored) |
| `downshift-theme@downshift` | [`downshift-theme/`](downshift-theme/README.md) | Downshift (original, Apache-2.0, optional, not installed by `setup`) |

The third mod, cache-tax, lives inside the plugin at
[`hooks/cache-tax/`](../hooks/cache-tax/README.md). None of them holds routing
logic; they need Claude Code 2.1.287 or later. On Claude Code `downshift setup` installs
savvy-progress, filetree and downshift-dock with `claude plugin install` (never by editing a settings
file); `--no-plugin` skips them.

## Install

```text
/plugin install savvy-progress@downshift
/plugin install filetree@downshift
/plugin install downshift-dock@downshift
```

## Why they are not inside the plugin

- A plugin takes one hooks module, and cache-tax, savvy-progress and filetree
  all hook `session.start` without a matcher, so they cannot share one plugin.
- A mod's `$.state` and its tool namespaces (`mcp__<plugin>__...`) are keyed by
  plugin name, so folding a mod into `downshift` would change them.

Hence one plugin per mod, listed in
[`.claude-plugin/marketplace.json`](../.claude-plugin/marketplace.json).

## Update procedure

Re-vendor from the pinned upstream commit; edit a vendored mod only through its
documented local patch (owner decision 2026-10-19): clone upstream,
check out the new tag, copy `.claude-plugin/plugin.json`, `hooks/`, `types/`,
tests and `LICENSE` byte for byte (filetree: drop its own `marketplace.json`),
compare with `sha256sum`, run the tests below, then update the version and
commit in the mod's README, in NOTICE and in CHANGELOG, then reapply the
mod's `downshift-local.patch` (`git apply`), re-run its tests and refresh the patch. Do not run anything
from the clone.

## Tests

Run each plugin on its own:

```bash
claude plugin validate mods/savvy-progress
claude plugin test mods/savvy-progress
claude plugin validate mods/filetree
claude plugin test mods/filetree
claude plugin validate mods/downshift-dock
claude plugin test mods/downshift-dock
```

A run from the repository root (`claude plugin test .`) picks the nested suites
and runs them against the root plugin, where they fail; use the per-plugin
commands above for the mods. The root run is still the one for cache-tax.
