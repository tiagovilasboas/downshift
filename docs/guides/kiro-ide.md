# Kiro IDE: Observation-Only Mode

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift-ai

Kiro IDE (the VS Code extension by AWS) spawns subagents through `invoke_sub_agent`. Unlike Claude
Code, **Kiro IDE does not support `updatedInput` for model rewriting**. The adapter operates in
observation-only mode: it computes routing decisions for telemetry but cannot enforce them at runtime.

For full evidence, see [kiro-observation-only-rea-2026-11-10.md](../evidence/kiro-observation-only-rea-2026-11-10.md).

## 1. What Kiro IDE can and cannot do

| Capability | Supported | Notes |
|------------|-----------|-------|
| Detect subagent spawn | ✅ | PreToolUse on `invoke_sub_agent` |
| Read task/prompt | ✅ | From `tool_input.prompt` |
| Compute recommended model | ✅ | For telemetry |
| Block spawn (exit 2) | ✅ | Documented in kiro.dev |
| Rewrite model via `updatedInput` | ❌ | Not supported (REA confirmed) |
| Read spawn's model field | ❌ | Field absent from payload |

Code: `internal/adapters/kiro/kiro.go`, `internal/core/capabilities.go` (`KiroCaps`).

## 2. Why Kiro cannot rewrite models

The Kiro hook protocol supports only two outcomes:

- **Exit 0:** Allow. Stdout is added to agent context.
- **Exit 2:** Block. Stderr is returned to the model.

There is no `updatedInput` mechanism. The official docs at kiro.dev/docs/hooks/actions/ describe
only exit codes. All `updatedInput` issues are in `anthropics/claude-code`, not `kirodotdev/Kiro`.

Additionally, the `invoke_sub_agent` payload has **no model field**. The model is decided internally
by Kiro, not exposed to hooks. See [kirodotdev/Kiro#6637](https://github.com/kirodotdev/Kiro/issues/6637).

## 3. What the adapter does instead

The Kiro adapter computes the routing decision (which model SHOULD be used) and records it for
telemetry, but returns no output to the hook:

```go
// Return empty output and no note: Kiro IDE is observation-only, so we only
// record telemetry. The shared runner will record OutcomeAllow because note
// is empty.
return allow(), "", decision
```

This lets you:
- Track "missed savings" in the dashboard
- See which spawns would have been routed differently
- Build a case for Kiro to add `updatedInput` support

## 4. Steering-based advisory (alternative approach)

Since Kiro cannot rewrite at runtime, consider **design-time routing** via agent definitions.
Projects like [kiro-with-harness](https://github.com/TeiNam/kiro-with-harness) achieve model routing
by having specialized agents with fixed models:

```markdown
| Tier | Model | Agents |
|------|-------|--------|
| Deep reasoning | claude-opus-5.5 | architect, security-reviewer |
| Balanced | claude-sonnet-5.5 | code-reviewer, refactor-cleaner |
| Cost-optimized | claude-haiku-5.5 | translator-docs, article-writer |
```

The orchestrator agent chooses which specialized agent to invoke based on the task. Each agent has
its model fixed in its `.kiro/agents/*.md` definition file.

### Creating a steering advisory hook

You can create a Kiro hook that **advises** (via context injection) without enforcing:

```json
{
  "version": "v1",
  "hooks": [{
    "name": "downshift-advisory",
    "trigger": "PreToolUse",
    "matcher": "invoke_sub_agent",
    "action": {
      "type": "agent",
      "prompt": "Before spawning this subagent, consider the task complexity. For simple lookups or searches, delegate to a cost-optimized agent. For architecture or security work, use a reasoning-focused agent. Check .kiro/agents/ for available specialized agents."
    }
  }]
}
```

This injects guidance into the model's context, nudging it toward better agent selection. It's
advisory, not enforcement: the model may or may not follow the suggestion.

## 5. Contrast with other harnesses

| Harness | Rewrite | Advisory | Telemetry |
|---------|---------|----------|-----------|
| Claude Code | ✅ `updatedInput` | ✅ | ✅ |
| Codex | ✅ `updatedInput` | ✅ | ✅ |
| Kiro IDE | ❌ | ✅ steering | ✅ |
| Kiro CLI V3 | ❌ | ✅ steering | ✅ |
| KiroCrew | ❌ | ✅ policy | ✅ |

## 6. Future work

If Kiro adds `updatedInput` support (or a model field to `invoke_sub_agent`), the adapter can be
upgraded. Monitor issues:

- [#6637](https://github.com/kirodotdev/Kiro/issues/6637): Allow custom subagents to honor model field
- [#4180](https://github.com/kirodotdev/Kiro/issues/4180): Option to define model for hooks/subagents

## 7. Installation

Kiro IDE uses the same Downshift binary. Install it following [install.md](install.md), then
configure the PreToolUse hook:

```json
{
  "version": "v1",
  "hooks": [{
    "name": "downshift-observe",
    "trigger": "PreToolUse",
    "matcher": "invoke_sub_agent",
    "action": {
      "type": "command",
      "command": "downshift hook kiro"
    }
  }]
}
```

Save as `.kiro/hooks/downshift.json`. The adapter will record telemetry for all spawns but will not
modify them. Use `downshift report` to see missed savings opportunities.
