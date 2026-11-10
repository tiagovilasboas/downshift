# Kiro IDE: Observation-Only Mode (REA 2026-11-10)

> **Status:** Confirmed. Kiro IDE does NOT support model rewrite via PreToolUse hooks.
>
> **Updated:** 2026-10-29 with additional evidence from kiro.dev docs and community repos.

## Summary

Reverse Engineering Analysis (REA) conducted on 2026-11-10 confirms that Kiro IDE's hook protocol does not support `updatedInput` for model rewriting. The adapter is now observation-only, similar to KiroCrew's policy mode.

## Evidence

### 1. Official Documentation (kiro.dev)

Direct fetches from kiro.dev/docs/hooks/* (2026-10-29):

- **Hook Actions** (https://kiro.dev/docs/hooks/actions/):
  > "If the command returns an exit code of '0' indicating success, the stdout output of the command is added to the agent's context."
  > "If the command returns any other exit code, the stderr output [...] is sent to the agent, and the agent is notified that the hook returned an error. Additionally, in the case of the Pre Tool Use hook, the tool invocation is blocked."

  **Only two outcomes:** allow (exit 0) or block (exit non-0). No `updatedInput`, no rewrite.

- **Hook Types** (https://kiro.dev/docs/hooks/types/):
  > "Triggers when the agent is about to invoke a tool. Can validate and block tool usage."

  Use cases listed: "Block certain tool invocations", "Validate tool inputs before execution", "Provide additional instructions". **No rewrite capability.**

- **Hook Schema** (https://kiro.dev/docs/hooks/):
  ```json
  {
    "version": "v1",
    "hooks": [{
      "name": "...",
      "trigger": "PreToolUse",
      "matcher": "...",
      "action": { "type": "command", "command": "..." }
    }]
  }
  ```
  **No `updatedInput` field in schema.**

### 2. Community Documentation (dev-rb-hub/rendu.ai)

Mirror of kiro.dev docs (fetched 2026-09-06) at https://github.com/dev-rb-hub/rendu.ai/blob/main/docs/reference/kiro-cli/hooks.md:

> "Exit 0: succeeded, STDOUT captured. For AgentSpawn/UserPromptSubmit the STDOUT is injected into context; for PreToolUse this is a **delivered 'allow'**."
> "Exit 2: deny, on the gating events (PreToolUse and UserPromptSubmit here [...])"

Confirms: PreToolUse exit 0 = allow, exit 2 = deny. **No updatedInput mechanism.**

### 3. everything-kiro Collection

https://github.com/iamaanahmad/everything-kiro (kept in sync with Kiro schemas):

> "Triggers: SessionStart, Stop, UserPromptSubmit, PreToolUse, PostToolUse, PreTaskExec, PostTaskExec, PostFileCreate, PostFileSave, PostFileDelete. PreToolUse, UserPromptSubmit, and PreTaskExec can block — a command action exiting with code 2 stops the operation."

34 hooks, 8 agents, full schema coverage. **Zero mention of `updatedInput` or model rewrite.**

### 4. GitHub Issues (kirodotdev/Kiro)

| Issue | Title | Status | Relevance |
|-------|-------|--------|-----------|
| [#6637](https://github.com/kirodotdev/Kiro/issues/6637) | IDE: Allow custom subagents to honor the model field | Open | **Confirms model field is ignored** for custom agents in IDE |
| [#4180](https://github.com/kirodotdev/Kiro/issues/4180) | Option to Define Model for Hooks and Subagents | Open | **No mechanism to define model** for hooks/subagents |
| [#7500](https://github.com/kirodotdev/Kiro/issues/7500) | Pass hook event JSON to IDE runCommand hooks | Open | **IDE hooks don't receive tool_input** via stdin |
| [#8754](https://github.com/kirodotdev/Kiro/issues/8754) | Per-agent reasoning effort inheritance | Open | Effort also doesn't propagate to subagents |
| [#12163](https://github.com/kirodotdev/Kiro/issues/12163) | Hook stdin missing model field | Open | **Confirms stdin has no model field** |

### 5. Contrast: Kiro vs Claude Code `updatedInput`

All search results for `updatedInput` lead to **Claude Code (anthropics/claude-code)**, not Kiro:

| Issue | Repository | Description |
|-------|------------|-------------|
| #77851 | anthropics/claude-code | PostToolUse cannot recover original tool_input after PreToolUse rewrite |
| #90725 | anthropics/claude-code | updatedInput replaces entire input, drops sibling fields |
| #75915 | anthropics/claude-code | updatedInput discarded when sibling hook returns "ask" |
| #102469 | anthropics/claude-code | updatedInput rewrite triggers local classifier |

**Zero `updatedInput` issues in kirodotdev/Kiro.** The feature doesn't exist in Kiro.

### 6. `permissionDecision` Analysis

The Kiro system prompt mentions `permissionDecision: "ask"` for PreToolUse, but:

1. **Not documented** in kiro.dev/docs/hooks/actions/ or /types/
2. **All issues** referencing `permissionDecision` are in:
   - anthropics/claude-code (#89561, #61918, #58971, etc.)
   - github/copilot-cli (#2643)
   - microsoft/vscode (#336812, #339589)
3. **Kiro official docs** only mention exit codes: 0=allow, 2=block

Hypothesis: `permissionDecision` may be an internal/undocumented feature inherited from the underlying architecture, but it's not publicly documented or reliably available.

### 7. Probe Test (2026-11-10)

Live probe hook captured `invoke_sub_agent` payload in Kiro IDE:

```json
{
  "hook_event_name": "PreToolUse",
  "tool_name": "invoke_sub_agent",
  "tool_input": {
    "name": "general-task-execution",
    "prompt": "...",
    "explanation": "...",
    "contextFiles": [...],
    "preset": null,
    "specTask": null
  }
}
```

**Missing fields:** `model`, `effort`, `reasoning_effort`

The model is decided internally by Kiro, not exposed to hooks.

### 8. Contrast with Claude Code

| Feature | Claude Code | Kiro IDE |
|---------|-------------|----------|
| `updatedInput` in stdout | ✅ Supported | ❌ Not supported |
| `permissionDecision` | ✅ allow/deny/ask (documented) | ⚠️ Undocumented |
| Model field in spawn payload | ✅ Present | ❌ Absent |
| Custom agent model honored | N/A | ❌ Ignored (#6637) |
| Hook rewrite capability | ✅ Full rewrite | ❌ None |

All `updatedInput` bugs and issues are in `anthropics/claude-code`, not `kirodotdev/Kiro`.

## Conclusion

Kiro IDE adapter must operate in **observation-only mode**:

1. **Can do:**
   - Detect spawns of subagent (PreToolUse/PostToolUse)
   - Compute and log recommended model (for telemetry)
   - Block spawn if needed (exit 2)
   - Record child end events

2. **Cannot do:**
   - Rewrite the model of a subagent spawn
   - Modify tool_input parameters
   - Force a different tier than Kiro's internal decision

## Changes Made

1. `internal/core/capabilities.go`: `KiroCaps.CanRewriteModel = false`
2. `internal/adapters/kiro/kiro.go`: Observation-only mode, no rewrite attempt
3. `docs/reference/harness-matrix.md`: Updated status and capability grid

## Open Source Projects Analysis (2026-10-29)

REA of existing model routing projects reveals how others approach this problem:

### Projects with Full Rewrite (Claude Code)

| Project | Language | Approach |
|---------|----------|----------|
| [nedjamez/claude-code-model-router](https://github.com/nedjamez/claude-code-model-router) | Python | Regex keyword matching for opus/sonnet/haiku |
| [leftspace89/jevsubrouter](https://github.com/leftspace89/jevsubrouter) | Node.js | External API (TypeSafe) for classification |
| [wannabefro/claude-tier-router](https://github.com/wannabefro/claude-tier-router) | Node.js | Policy-driven defaults |

All use `hookSpecificOutput.updatedInput` to rewrite the model field. This is **only possible on Claude Code**.

### Projects for Kiro (No Rewrite)

| Project | Approach |
|---------|----------|
| [TeiNam/kiro-with-harness](https://github.com/TeiNam/kiro-with-harness) | **Design-time agent definitions** with fixed models per role |
| [NachoFLizaur/oh-my-kiro](https://github.com/NachoFLizaur/oh-my-kiro) | Multi-agent orchestration (Phantom/Revenant/Wraith) |
| [davide-desio-eleva/kirograph](https://github.com/davide-desio-eleva/kirograph) | Knowledge graph, not routing |

**Key insight from kiro-with-harness:** Model routing is achieved via **steering + agent definitions**, not runtime hooks:

```markdown
| Tier | Model | Agents |
|------|-------|--------|
| Deep reasoning | claude-opus-5.5 | orchestrator, architect, security-reviewer |
| Balanced | claude-sonnet-5.5 | code-reviewer, refactor-cleaner |
| Cost-optimized | claude-haiku-5.5 | translator-docs, article-writer |
```

The orchestrator agent chooses which specialized agent to invoke based on the task. Each agent has its model fixed in its definition file.

### Implications for Downshift

| Harness | Can Rewrite | Implementation |
|---------|-------------|----------------|
| Claude Code | ✅ Yes | `updatedInput` in PreToolUse |
| Codex | ✅ Yes | Same as Claude Code (requires `permissionDecision`) |
| Kiro IDE | ❌ No | Observation + steering advisory |
| Kiro CLI V3 | ❌ No | Same as IDE |
| KiroCrew | ❌ No | Policy mode (block only) |

### Recommended Approach for Kiro

Since Kiro cannot rewrite models at runtime, Downshift should:

1. **Telemetry:** Record what model SHOULD have been used (already implemented)
2. **Steering Advisory:** Inject guidance via hook `agent` action
3. **Dashboard:** Show "missed savings" opportunities

## Future Work

If Kiro adds `updatedInput` support (or a model field to `invoke_sub_agent`), the adapter can be upgraded. Monitor issues #6637 and #4180.
