# Kiro IDE: Observation-Only Mode (REA 2026-11-10)

> **Status:** Confirmed. Kiro IDE does NOT support model rewrite via PreToolUse hooks.

## Summary

Reverse Engineering Analysis (REA) conducted on 2026-11-10 confirms that Kiro IDE's hook protocol does not support `updatedInput` for model rewriting. The adapter is now observation-only, similar to KiroCrew's policy mode.

## Evidence

### 1. Official Documentation (kiro.dev)

- **Hook Actions** (https://kiro.dev/docs/hooks/actions/): Documents only exit code semantics (0=allow, 2=block), no `updatedInput` mechanism.
- **Hook Types** (https://kiro.dev/docs/hooks/types/): PreToolUse can "validate and block tool usage" but no rewrite capability documented.
- **CLI V3 Migration** (https://kiro.dev/docs/cli/v3/hooks-migration/): No mention of `updatedInput` anywhere in Kiro docs.

### 2. GitHub Issues (kirodotdev/Kiro)

| Issue | Title | Status | Relevance |
|-------|-------|--------|-----------|
| [#6637](https://github.com/kirodotdev/Kiro/issues/6637) | IDE: Allow custom subagents to honor the model field | Open | **Confirms model field is ignored** for custom agents in IDE |
| [#4180](https://github.com/kirodotdev/Kiro/issues/4180) | Option to Define Model for Hooks and Subagents | Open | **No mechanism to define model** for hooks/subagents |
| [#7500](https://github.com/kirodotdev/Kiro/issues/7500) | Pass hook event JSON to IDE runCommand hooks | Open | **IDE hooks don't receive tool_input** via stdin |
| [#8754](https://github.com/kirodotdev/Kiro/issues/8754) | Per-agent reasoning effort inheritance | Open | Effort also doesn't propagate to subagents |

### 3. Probe Test (2026-11-10)

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

### 4. Contrast with Claude Code

| Feature | Claude Code | Kiro IDE |
|---------|-------------|----------|
| `updatedInput` in stdout | ✅ Supported | ❌ Not supported |
| `permissionDecision` | ✅ allow/deny/ask | ⚠️ allow/deny only (ask undocumented) |
| Model field in spawn payload | ✅ Present | ❌ Absent |
| Custom agent model honored | N/A | ❌ Ignored (#6637) |

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

## Future Work

If Kiro adds `updatedInput` support (or a model field to `invoke_sub_agent`), the adapter can be upgraded. Monitor issues #6637 and #4180.
