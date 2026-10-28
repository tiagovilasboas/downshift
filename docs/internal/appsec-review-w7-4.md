# W7.4 hook-path security: self-review draft

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

**Date:** 2026-10-28
**Status:** **Self-review draft, not a gate.** Written by the implementing
session, not by an independent reviewer. W7.4 stays open until an independent
AppSec review (for example, the `appsec-pr-review` skill run by a separate
agent or person) returns PASS on the diff `origin/main...downshift/mvp-ponto-zero`.

## Scope

PreToolUse hook path and decider v2 cut, default profile table (W7.1), economy
mode (W7.9), `dead_ends.json` locking, `downshift loop`, setup changes
(force env, stale-binary prune, policy init).

## Checked, with evidence

| Check | Evidence | Result |
|---|---|---|
| Panic in a hook is recovered | `recover()` at `cmd/downshift/hookrun.go` lines 191, 374, 527 | Present |
| No new `panic`, `os.Exit` or `log.Fatal` on the routing path | grep of the diff `959d34b7..HEAD` over `cmd/downshift/decider.go` and `internal/routing/` | None added |
| Unusable policy does not move a spawn | `unchangedInput` drops the agent type; `TestRoutingDeciderDefaultProfile` (invalid and malformed policy cases) | Tested |
| Risk never goes to the small tier | `TestDefaultProfilesInvariants`; economy tests with a `secret` risk signal | Tested |
| `dead_ends.json` writes across processes | directory lock plus temp file and rename in `internal/cascade/deadends.go` | Present; no multi-process stress test |
| Loop sensor runs without a shell | `exec.CommandContext(ctx, parts[0], parts[1:]...)` in `internal/loop/runner.go` | No shell; the argument split is a plain whitespace split |
| No new `0644`/`0755` writes in non-test code of the diff | grep of the diff `959d34b7..HEAD`, test files excluded | None found |

## Not checked

- Prompt privacy of new telemetry fields end to end (only the existing
  invariant tests ran).
- Stale-binary prune (`setup_prune.go`): what it may delete when PATH holds a
  foreign `downshift` binary, beyond its own tests.
- `downshift loop --sensor`: the command comes from the operator; no threat
  model for a sensor string supplied by an agent.
- Windows behaviour of the lock (only `GOOS=windows go vet` ran, no tests).
- ASVS mapping of each finding.
