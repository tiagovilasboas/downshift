# Policy Guide

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

The orchestrator chooses each subagent's model; Downshift applies that choice deterministically within the policy you declare in local policy files (`.downshift/policy.json`): profiles, ordered rules, quota gates and tier floors. The built-in risk floor was removed on 2026-10-28 (O8); declare a floor here if a project needs one.

## Initializing Project Policy

To generate a starting project policy with full schema definitions, run:

```bash
downshift policy init
```

This runs a dry-run preview showing the configuration and asks for confirmation before writing `.downshift/policy.json`.

## No built-in table (owner decision O8)

Downshift ships no default model for any agent type. The orchestrator chooses the model and the effort of each
subagent; the defaults that used to live in the table (`Explore`, `claude-code-guide`, `statusline-setup` on small)
are now guidance in the `downshift-orchestrator` playbook, which `downshift setup` installs. A downshift happens
when your `policy.json` declares it (a rule, a profile), when the orchestrator sends a tier hint, or, when none of
those decided, when the task class is a decisive `TRIVIAL` or `SIMPLE` with no risk category (on by default since
2026-11-01; `routing.by_class: false` turns it off, see [config.md](config.md#routing-by-task-class-on-by-default)).

## Economy mode (removed 2026-11-02)

`downshift policy economy` wrote a top-level `"mode": "economy"` key that no routing step read, so it never changed a
model; the `DOWNSHIFT_MODE` variable and the `mode.json` state file were not read either. The command now prints a
removal notice and writes nothing. A `"mode"` key already in `policy.json` is ignored and still loads; you can delete
it. To move simple work to a smaller tier, keep class routing on (the default) or declare a rule or profile.
