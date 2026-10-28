# Policy and Economy Mode Guide

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

Downshift provides deterministic subagent routing governed by local policy files (`.downshift/policy.json`).

## Initializing Project Policy

To generate a starting project policy with full schema definitions, run:

```bash
downshift policy init
```

This runs a dry-run preview showing the configuration and asks for confirmation before writing `.downshift/policy.json`.

## No built-in table (owner decision O8)

Downshift ships no default model for any agent type. The orchestrator chooses the model and the effort of each
subagent; the defaults that used to live in the table (`Explore`, `claude-code-guide`, `statusline-setup` on small)
are now guidance in the `downshift-orchestrator` playbook, which `downshift setup` installs. A downshift happens only
when your `policy.json` declares it (a rule, a profile or a tier floor), or when the orchestrator sends a tier hint.

## Economy Mode (W7.9)

Economy mode applies to the declared policy only. It no longer lowers built-in agent types, because there is no
built-in table (O8). `explicit_only` models are never chosen automatically.

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
