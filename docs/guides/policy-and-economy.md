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
| `general-purpose` | `mid` (Sonnet) when the classifier is confident the task is below COMPLEX; otherwise keep requested | Keep requested |
| `Plan` | Keep requested | Keep requested |

`general-purpose` is the catch-all type, so it inherits the main session's model. Without the class step a read-only review spawned from an Opus session ran on Opus. The class step only moves down, never below `mid`, and never on doubt: an unconfident or COMPLEX class, or any risk signal, keeps the requested model. A bounded brief (80+ words that state a read-only scope and a reply budget such as `Output (max 300 words)`) is a confident MEDIUM on its own.

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
