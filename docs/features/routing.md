# Subagent Model Delegation

> **Status: stable on Claude Code; functional on Codex and Cursor.**

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift-ai

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
5. The **task class** (on by default since 2026-11-01): a decisive `TRIVIAL` or `SIMPLE` class with no risk
   category moves the spawn down to `small` (routing note `verdict: "class"`). `MEDIUM` and `COMPLEX` never move
   down. `routing.by_class: false` in `policy.json` turns this step off; `routing.raise_complex: true` (off by
   default) lets a decisive `COMPLEX` task asked on `small` move up to `mid`, and nothing higher.
6. Otherwise the **requested model is kept**.

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
- Economy mode: `downshift policy economy` was removed on 2026-11-02, because nothing read the key it wrote.
- `CLAUDE_CODE_SUBAGENT_MODEL_FORCE=1` is no longer written by setup, because it overrides the per-call model the
  orchestrator chose.

## Guidance to the orchestrator and its workers

- **Orchestrator skill** (implemented): `downshift-orchestrator`, installed by `downshift setup` beside
  `downshift-audit`.
- **Playbooks** (implemented): `downshift playbook list|show|edit|add|remove|restore|check`. General always
  applies; Backend and Frontend ship. Rules are instructions to an agent, never enforced.
- **MCP server** (`downshift mcp serve`, implemented): 13 tools. Six inform the orchestrator: `providers` (the models this
  harness offers now), `playbook`, `status` (quota advice plus this session's delegated workers), `apply` (what would be applied for a choice), `plan` and `retrieve`.
  Seven run workers: `delegate`, `collect`, `cancel`, `diff`, `send`, `steer` and `release`. A worker keeps the tier,
  effort, directory and mode it started with on every turn; nothing climbs a tier. See
  `docs/internal/plans/mcp-delegation-spec.md` and `docs/internal/plans/mcp-worker-continuation.md`.

## Inspecting decisions

```bash
downshift session --why    # why the latest decision was made, in plain words
downshift stats            # decisions and token savings
```

The classifier (complexity class, risk categories) decides only step 5 above, and only to move simple work down.
It is also telemetry and the key of the floors you declare. See [classification](classification.md).

## Harness support

Whether a harness can honor a rewrite varies; the per-harness facts are in the
[harness matrix](../reference/harness-matrix.md).

## Pause

Every spawn is routed. To leave the spawns of one session unrouted, pause it with an explicit command:

```bash
downshift session --pause
downshift session --resume
```

The marker expires after 12 hours. See [session-view.md](../guides/session-view.md).
