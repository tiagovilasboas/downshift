# Policy and Economy Mode Guide

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

Downshift provides deterministic subagent routing governed by local policy files (`.downshift/policy.json`).

## Initializing Project Policy

To generate a starting project policy with full schema definitions, run:

```bash
downshift policy init
```

This runs a dry-run preview showing the configuration and asks for confirmation before writing `.downshift/policy.json`.

## Default Profiles Table (W7.1)

Out of the box without any policy file, Downshift applies the following defaults for built-in Claude Code subagent types:

| Agent Type | Default Tier | With Risk Signal |
|---|---|---|
| `Explore` | `small` (Haiku) | `mid` (Sonnet) |
| `claude-code-guide` | `small` (Haiku) | `mid` (Sonnet) |
| `statusline-setup` | `small` (Haiku) | `mid` (Sonnet) |
| `general-purpose` | Keep requested | Keep requested |
| `Plan` | Keep requested | Keep requested |

## Economy Mode (W7.9)

Economy mode lowers every agent type by one relative tier:
- Standard types drop from `mid` to `small`.
- Types keeping requested models drop one tier below the parent session model.
- **Safety Bounds:** Risk signals never drop below `mid`. `explicit_only` models are never chosen automatically.

### Enabling Economy Mode

```bash
# Check current economy mode status
downshift policy economy

# Enable economy mode
downshift policy economy on

# Disable economy mode
downshift policy economy off
```

Alternatively, add `"mode": "economy"` directly to `.downshift/policy.json`.
