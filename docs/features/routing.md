# Subagent Model Delegation

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

**The orchestrator chooses, Downshift applies, your policy bounds it, and Downshift measures** (owner decision
O8, 2026-10-28). The lead session picks agent, model and effort for every delegation. Downshift never picks a
model on its own. Its own code never calls an LLM, adds no token cost to the hot path and fails open.

## How it works

When your harness spawns a subagent, Downshift's PreToolUse hook:

1. **Reads the spawn payload** (the requested model, agent type, and the orchestrator's optional hints).
2. **Applies the precedence below**, using only what you declared in `policy.json` and what the orchestrator sent.
3. **Returns the result**: the requested model rewritten to the chosen tier, or the request unchanged.

Precedence, first match wins:

1. A **pinned model** is kept.
2. A **declared profile**, named by the request or matching the agent type.
3. The **first matching declared rule** (kind, effort, quota gates), in declaration order.
4. The orchestrator's **tier hint** (`[tier=small|mid|frontier]`), only to move **down** for now. Moving up is
   specified, not built.
5. Otherwise the **requested model is kept**.

**Doubt keeps.** When Downshift cannot read the request, the requested model stays (fail-open).

## Refusals

A model id the harness catalog does not know, and a model marked `explicit_only`, are never written. The hook
changes only `model` and `reasoning_effort`; sibling fields are preserved.

## What your policy declares

`downshift policy init` creates `.downshift/policy.json`; `downshift policy check` validates it. It holds profiles,
ordered rules, quota gates and tier floors. A tier floor is the way to say that work in some area must never go
small. This is declared data, not inference.

## What was removed (2026-10-28, O8)

- The built-in agent-type table (N11: Explore, claude-code-guide, statusline-setup on small). Its content is now
  guidance in the `downshift-orchestrator` skill that `downshift setup` installs.
- The automatic upgrade after a recorded failure. The orchestrator decides the next model.
- The built-in risk floor and veto. **Residual risk:** the protection is now the orchestrator's judgment, guided
  by the playbook, plus the floors you declare. Without a declared floor, nothing in Downshift stops a risky task
  from running on a small model.
- The economy mode effect on built-in types (`downshift policy economy` still exists; removal is pending).
- `CLAUDE_CODE_SUBAGENT_MODEL_FORCE=1` is no longer written by setup, because it overrides the per-call model the
  orchestrator chose.

## Guidance to the orchestrator and its workers

- **Orchestrator skill** (implemented): `downshift-orchestrator`, installed by `downshift setup` beside
  `downshift-audit`.
- **Playbooks** (implemented): `downshift playbook list|show|edit|add|remove|restore|check`. General always
  applies; Backend and Frontend ship. Rules are instructions to an agent, never enforced.
- **MCP server** (`downshift mcp serve`, implemented): four information tools for the orchestrator, `providers` (the models this
  harness offers now), `playbook`, `status` (quota advice) and `apply` (what would be applied for a choice). The tools that
  start workers (`delegate`, `steer`, `result`, `cancel`) are specified, not built. See
  `docs/internal/plans/mcp-delegation-spec.md`.

## Inspecting decisions

```bash
downshift session --why    # why the latest decision was made, in plain words
downshift stats            # decisions and token savings
```

The classifier (complexity class, risk categories) no longer decides. It remains as telemetry, as the key of
floors you declare, and in DCI. See [classification](classification.md).

## Harness support

Whether a harness can honor a rewrite varies; the per-harness facts are in the
[harness matrix](../reference/harness-matrix.md).

## Disable

```bash
export DOWNSHIFT_NO_ROUTE=1
```
