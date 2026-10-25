# Spike: KiroCrew Harness Detection & Intelligence Index

**Date:** 2026-10-25
**Priority:** Urgent
**Status:** T1 COMPLETE ✅

## Problem Statement

KiroCrew is not being properly identified in Downshift's exec-events.jsonl.
Current distribution from the downshift session:
- `unknown`: 472 events
- `null`: 505 events  
- `claude-code`: 256 events

KiroCrew events are falling into `unknown` or `null` because:
1. `KiroCrewCaps` doesn't have `SessionEnv` configured
2. The `envHarness()` function can't detect KiroCrew environment

## Root Cause Analysis

### 1. Missing SessionEnv in KiroCrewCaps

**File:** `internal/core/capabilities.go`

Current KiroCrewCaps:
```go
KiroCrewCaps = HarnessCapabilities{
    CanRewriteModel: false,
    CanApplyEffort: false,
    ChildEndEvent: "postToolUse",
    UsageSource: UsageNone,
    CanAdviseShellWrap: true,
}
```

**Missing:** `SessionEnv: "KIRO_SESSION_ID"`

### 2. KiroCrew Environment Variables Available

From `env | grep KIRO`:
```
KIRO_SESSION_ID=70156318-1c75-44f0-ab31-e6adb7ebe0f5
KIRO_CLI_ACP_CLIENT_NAME=kirocrew
KIRO_CLI_CLIENT_APPLICATION=kirocrew
KIRO_NO_AUTO_UPDATE=1
KIROCREW_SANDBOX_ACTIVE=1
KIROCREW_PROJECT_DIR=/Applications/KiroCrew.app/Contents/Resources
KIROCREW_BOUND_PORT=5476
KIROCREW_SPAWN_INSTANCE=7c3ac334b3cc47ca
KIROCREW_RUNTIME_PYTHON=...
```

### 3. envHarness() Function

**File:** `cmd/downshift/session.go:130-143`

```go
func envHarness() string {
    for _, n := range core.AllHarnessCaps() {
        if n.Caps.SessionEnv == "" {
            continue  // KiroCrew skipped here!
        }
        if os.Getenv(n.Caps.SessionEnv) != "" {
            return n.Harness
        }
    }
    return "unknown"
}
```

## KiroCrew Models (from kiro-cli)

Available models with cost multipliers:

| Model | Context | Multiplier | Tier |
|-------|---------|------------|------|
| claude-opus-4.5 | 200k | 2.2x | frontier |
| claude-sonnet-4.6 | 1M | 1.3x | mid |
| claude-sonnet-4.5 | 200k | 1.3x | mid |
| claude-haiku-4.5 | 200k | 0.4x | small |
| deepseek-3.2 | 164k | 0.25x | small |
| minimax-m2.5 | 196k | 0.25x | small |
| minimax-m2.1 | 196k | 0.15x | small |
| glm-5 | 200k | 0.5x | mid |
| qwen3-coder-next | 256k | 0.05x | small |
| auto | 1M | 1.0x | (router) |

**Downshift opportunity:** Route subagent spawns from opus (2.2x) or sonnet (1.3x)
to haiku (0.4x), deepseek (0.25x), or qwen3 (0.05x) for trivial tasks.

## Tasks

### T1: Add SessionEnv to KiroCrewCaps (P0) ✅ DONE

**File:** `internal/core/capabilities.go`

```go
KiroCrewCaps = HarnessCapabilities{
    CanRewriteModel: false,
    CanApplyEffort: false,
    ChildEndEvent: "postToolUse",
    UsageSource: UsageNone,
    CanAdviseShellWrap: true,
    SessionEnv: "KIRO_SESSION_ID",  // ADDED
}
```

**Tests added:**
- `cmd/downshift/exec_session_test.go` — 2 new cases for KiroCrew (direct + hook)
- `cmd/downshift/watch_test.go` — `clearAllSessionEnvs(t)` helper to prevent pollution
- `cmd/downshift/session_test.go` — clear all SessionEnv vars in priority test

**Verification:**
```bash
cd cmd/downshift && go build -o /tmp/ds . && /tmp/ds compactor -- git status
tail -1 ~/.downshift/exec-events.jsonl | jq '{harness}'
# Output: {"harness": "kirocrew"}
```

### T2: Add PayloadMarkers for KiroCrew (P1)

Unique KiroCrew payload fields from audit.log:
- `kiro_hooks`
- `tool_tags` containing `@builtin`

```go
PayloadMarkers: []string{"kiro_hooks", "kirocrew_spawn_instance"},
```

### T3: Verify exec-events harness field (P0)

After T1, run compactor from KiroCrew shell and verify:
```bash
tail -1 ~/.downshift/exec-events.jsonl | jq .harness
# Should output: "kirocrew" not "unknown"
```

### T4: Add KiroCrew models to catalog (P1)

Ensure all kiro-cli models are in the embedded catalog with correct tiers:

| ID | Tier | Status |
|----|------|--------|
| claude-opus-4.5 | frontier | check |
| claude-sonnet-4.6 | mid | check |
| claude-sonnet-4.5 | mid | check |
| claude-haiku-4.5 | small | check |
| deepseek-3.2 | small | add if missing |
| minimax-m2.5 | small | add if missing |
| minimax-m2.1 | small | add if missing |
| glm-5 | mid | add if missing |
| qwen3-coder-next | small | add if missing |

### T5: Add InferSession for KiroCrew (P2)

Consider if KiroCrew should have `InferSession: true`. The presence of
`KIRO_SESSION_ID` proves access to the account's model set.

### T6: Update harness-matrix.md (P2)

Document KiroCrew's new capabilities:
- SessionEnv: KIRO_SESSION_ID
- Compactor stats attribution works
- Session-scoped exec metrics available

### T7: Add AgentTypeHints for KiroCrew (P2)

If KiroCrew spawn_run payloads include agent type info, map them:
```go
AgentTypeHints: map[string]Tier{
    "senior-staff-ai-finops": TierFrontier,
    "senior-staff-fullstack": TierMid,
    "senior-staff-qa": TierMid,
    // etc.
},
```

## Intelligence Index Opportunity

With harness detection fixed, we can:

1. **Per-harness compactor stats:**
   - KiroCrew: X commands, Y bytes saved
   - Claude Code: A commands, B bytes saved

2. **Session-scoped metrics:**
   - Group exec events by KIRO_SESSION_ID hash
   - Show savings per KiroCrew session

3. **Model routing intelligence:**
   - Track which KiroCrew subagents run which models
   - Recommend spawn_run model overrides for trivial tasks

4. **Cross-harness comparison:**
   - Compare compaction rates: KiroCrew vs Claude Code
   - Identify which harness benefits most from downshift

## Acceptance Criteria

- [ ] `envHarness()` returns `"kirocrew"` when `KIRO_SESSION_ID` is set
- [ ] exec-events.jsonl records `harness: "kirocrew"` for compactor calls
- [ ] `downshift stats` shows KiroCrew as a separate harness
- [ ] All kiro-cli models resolve in the catalog
- [ ] Tests pass: `go test ./...`

## References

- `internal/core/capabilities.go:107-113` — KiroCrewCaps definition
- `cmd/downshift/session.go:130-143` — envHarness() function
- `cmd/downshift/exec.go:59-60` — ev.Harness = envHarness()
- `internal/discovery/sources.go:84-117` — KiroCLI discovery
