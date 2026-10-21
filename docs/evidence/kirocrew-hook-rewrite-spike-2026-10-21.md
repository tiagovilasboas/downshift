# KiroCrew Hook Rewrite Spike: PostToolUse Output Replacement

**Date:** 2026-10-21
**Status:** Concluded - Gap Confirmed
**Author:** Tiago Vilas Boas
**Related:** Downshift Transparent Compaction

## Executive Summary

This spike investigated whether KiroCrew's `PreToolUse` or `PostToolUse` hooks support
command/output rewriting via an `updated_input` mechanism, as exists in Claude Code.
**Conclusion: Neither hook supports rewriting. The gap is architectural.**

## Background

Downshift's steering-based compaction requires agents to manually wrap commands:

```bash
downshift compactor -- go test ./...
```

For transparent (native) compaction, Downshift needs a hook that can intercept and rewrite
shell commands before execution, or replace tool output after execution.

## Investigation

### 1. Official Kiro Documentation (kiro.dev/docs/hooks)

**PreToolUse Contract:**
- Exit 0: Hook succeeded, stdout added to agent's context
- Exit 2: Block tool (deny)
- Other: Show stderr warning to user

**PostToolUse Contract:**
- Exit 0: Hook succeeded, stdout **added to** agent's context
- Other: Show stderr warning to user

**Critical finding:** Neither contract mentions `updated_input`, `rewrite`, `replace`,
or any mechanism to modify tool input/output. Stdout is **appended**, not substituted.

### 2. KiroCrew Source Code Analysis

**File:** `src/kiro_crew/hook_runtime/script_hooks.py`

```python
@dataclass
class ScriptHookResult:
    """Result of executing a script hook."""
    hook_id: str
    hook_name: str
    event: str
    stdout: str = ""
    stderr: str = ""
    exit_code: int = -1
    error: str = ""
    duration_ms: int = 0

    @property
    def blocked(self) -> bool:
        """PreToolUse exit code 2 = block tool."""
        return self.exit_code == 2

    @property
    def succeeded(self) -> bool:
        return self.exit_code == 0
```

**Observation:** `ScriptHookResult` has no `updated_input`, `replacement_output`, or
similar field. The data model fundamentally does not support rewriting.

**File:** `src/kiro_crew/hook_runtime/hook_dispatch.py`

```python
async def fire_tool_hooks(
    hook_store: ScriptHookStore | None,
    event_title: str,
    event_tool_input: str | None = None,
    ...
) -> None:
    """Fire PreToolUse hooks for an EVENT_TOOL_CALL event.

    PostToolUse is NOT fired here because EVENT_TOOL_CALL is a notification
    that the tool is starting - the tool hasn't completed yet.

    Note: For EVENT_TOOL_CALL, hooks are informational only. The tool is
    already running (auto-approved by kiro-cli), so hook results cannot
    block execution.
    """
```

**Critical finding:** "hooks are informational only" - confirms PreToolUse cannot
modify the command that is about to run.

### 3. KiroCrew GitHub Issues Analysis

**Issue #13926:** "Agent spec hooks: remaining gaps after KAS delivery"

This issue documents known hook limitations:

1. **Auto-approved tool calls bypass PreToolUse hooks** - calls that are auto-approved
   never reach Crew's gate, so hooks cannot block them
2. **PostToolUse spec hooks on KAS still match the title** - no tool id available
3. **Matcher vocabulary differs across harnesses** - kiro-cli vs KAS tool names differ

**Key quote from maintainer @iamwhatever:**
> "For EVENT_TOOL_CALL, hooks are informational only. The tool is already running
> (auto-approved by kiro-cli), so hook results cannot block execution."

### 4. Claude Code Comparison

Claude Code has `updated_input` support in hooks:
- Issues #77851, #90725, #79321 reference `updated_input` bugs
- This is a **Claude Code-specific feature**, not present in Kiro/KiroCrew

## Why PostToolUse Cannot Replace Output

```
[BEFORE HOOK]
tool_response = 500 lines

[HOOK RUNS - stdout = 20 lines summary]

[AFTER HOOK]
tool_response = 500 lines + 20 lines hook stdout = 520 lines

RESULT: More tokens consumed, not fewer
```

The hook's stdout is **appended to context**, not substituted for the tool response.
There is no field or protocol to say "use this instead of the original output".

## Architecture of the Gap

```
┌──────────────────────────────────────────────────────────────┐
│                    KiroCrew Hook System                       │
├──────────────────────────────────────────────────────────────┤
│                                                              │
│  PreToolUse                                                  │
│  ┌────────────┐     ┌────────────┐     ┌────────────┐       │
│  │ Tool Call  │────▶│   Hook     │────▶│  Decision  │       │
│  │  (input)   │     │  Process   │     │ allow/deny │       │
│  └────────────┘     └────────────┘     └────────────┘       │
│                           │                                  │
│                           │ stdout → appended to context     │
│                           │ (NO modification of tool_input)  │
│                           ▼                                  │
│                      ┌────────────┐                          │
│                      │ Tool Runs  │                          │
│                      │ UNCHANGED  │                          │
│                      └────────────┘                          │
│                                                              │
│  PostToolUse                                                 │
│  ┌────────────┐     ┌────────────┐     ┌────────────┐       │
│  │ Tool Result│────▶│   Hook     │────▶│  Context   │       │
│  │  (output)  │     │  Process   │     │  Append    │       │
│  └────────────┘     └────────────┘     └────────────┘       │
│                           │                                  │
│                           │ stdout → appended to context     │
│                           │ (NO replacement of tool_output)  │
│                           ▼                                  │
│                      ┌────────────┐                          │
│                      │   Agent    │                          │
│                      │ Sees BOTH  │                          │
│                      └────────────┘                          │
│                                                              │
└──────────────────────────────────────────────────────────────┘
```

## Available Paths for Transparent Compaction

| Path | Status | Mechanism |
|------|--------|-----------|
| A. `updated_input` in PreToolUse | ❌ Not supported | Would need Kiro feature request |
| B. `shell-init` aliases | ✅ PR #392 merged | Agent's shell inherits aliases |
| C. PostToolUse output rewrite | ❌ Spike failed | stdout appends, doesn't replace |
| D. Upstream feature request | ⏳ Recommended | Request `updated_input` support |

## Recommendation

### Short-term: Continue with `shell-init` (Path B)

PR #392's `shell-init` command works:
- Agent's shell session sources Downshift aliases
- All allowlisted commands get compacted transparently
- Limitation: Only works when shell inherits the aliases

### Long-term: Feature Request on kirodotdev/KiroCrew

Request `updated_input` support for PreToolUse hooks:

**Proposed contract:**
```json
{
  "hook_event_name": "PreToolUse",
  "tool_name": "shell",
  "tool_input": {"command": "go test ./..."},
  "decision": "rewrite",
  "updated_input": {"command": "downshift compactor -- go test ./..."}
}
```

**Exit codes:**
- Exit 0: Allow tool as-is
- Exit 1: Warn and allow
- Exit 2: Block tool (existing)
- Exit 3 (new): Rewrite tool_input from stdout JSON

This would enable:
- Transparent output compaction
- Command transformation (security scanning, audit logging)
- Input normalization and validation

## Sources Consulted

1. **Official Kiro Docs:**
   - https://kiro.dev/docs/hooks/ (Schema v1)
   - https://kiro.dev/docs/hooks/types/ (PreToolUse/PostToolUse specs)
   - https://kiro.dev/docs/hooks/actions/ (Exit code contracts)

2. **KiroCrew GitHub:**
   - https://github.com/kirodotdev/KiroCrew (4.5k stars, Apache 2.0)
   - `src/kiro_crew/hooks.py` (hook constants and imports)
   - `src/kiro_crew/hook_runtime/script_hooks.py` (ScriptHookResult dataclass)
   - `src/kiro_crew/hook_runtime/hook_dispatch.py` (fire_tool_hooks implementation)
   - Issue #13926: "Agent spec hooks: remaining gaps after KAS delivery"

3. **Claude Code GitHub:**
   - Issues #77851, #90725, #79321 (confirm `updated_input` is CC-specific)

4. **Architecture Docs:**
   - `docs/system-specs/modules/memory-skills-hooks.md` (610KB, detailed)

## Conclusion

The gap is confirmed and architectural. KiroCrew's hook system was designed for
allow/block decisions and context enrichment, not for command/output transformation.
Native transparent compaction requires an upstream feature request to extend the
PreToolUse contract with `updated_input` support.

---

**Next steps:**
1. ✅ Document spike results (this file)
2. ✅ Open feature request on kirodotdev/KiroCrew - [#22767](https://github.com/kirodotdev/KiroCrew/issues/22767)
3. ✅ Continue with `shell-init` as working alternative
4. ⏳ Update roadmap with confirmed paths
