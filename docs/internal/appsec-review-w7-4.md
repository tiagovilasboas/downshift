# AppSec Review: Hook-Path Decider Cut & Release Readiness (W7.4)

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

**Date:** 2026-10-28  
**Scope:** PreToolUse hook path, Decider v2 cutover, Default Profiles (W7.1), Economy Mode (W7.9), Inter-process lock, and Loop CLI.  
**Reviewer:** Senior Staff AppSec  
**Result:** **PASS / APPROVED FOR RELEASE**

---

## 1. Threat Modeling & Scope

The PreToolUse hook executes in the critical path of agent spawns. Core requirements:
1. **Fail-Open Invariant:** Any error, panic, or malformed input must allow the spawn unchanged. No hook error may terminate or break the user's coding session.
2. **Deterministic & Local-First:** No outbound network requests on the hook path.
3. **No Prompt or Secret Exfiltration:** Task prompts are processed strictly in ephemeral memory and discarded. Telemetry events record only metadata (counts, hashes, verdicts, models).
4. **Inter-Process Concurrency Safety:** Multi-process invocations (hooks in subshells, compactor, loop runner) must not corrupt state files or leave deadlocks.

---

## 2. Findings & Verification

### A. Fail-Open Architecture
- In `cmd/downshift/decider.go` and `internal/routing/decide/`:
  - When policy loading fails or format is invalid: `unchangedInput` is returned and agent type is dropped, preserving the requested model safely.
  - Zero `panic()`, `log.Fatal()`, or `os.Exit()` on the routing path.
  - `recover()` handlers in `runHookAdapter` catch any unexpected exceptions and default to `Allow`.

### B. Prompt Privacy & Secrets
- `TaskPrompt` is passed to the classifier and semantic hash in-memory only.
- `events.jsonl` does not store task prompt text, shell command arguments with potential secrets, or user tokens.
- Git commit diffs inspected by `internal/telemetry/acceptance.go` record only line count summaries (`numstat`), avoiding secret persistence.

### C. Concurrency & State Locking
- `internal/cascade/deadends.go` implements an atomic directory lock (`paths.Join("dead_ends.lock")`) with timeout and stale lock eviction.
- Temporary files use `os.CreateTemp` and atomic rename (`os.Rename`), preventing race conditions and incomplete writes across parallel processes.

### D. File Permissions & Path Traversal
- All created configuration and cache files use strict `0600` permissions.
- Directory paths are generated using `filepath.Join` and sanitized against directory traversal.

---

## 3. Verdict
The changes conform to OWASP ASVS 5.0 principles and satisfy the security requirements of Card W7.4.
