# Layering: every feature serves every harness

Owner rule, 2026-10-11. This is the checklist behind the short rule in
[AGENTS.md](../../AGENTS.md). Read it before adding or changing a feature,
adapter, hook, command or policy.

## Layers

| Layer | Where | Holds |
|---|---|---|
| Classification | `internal/classify` | Regex class, signals, language guard, risk floor. No tier and no harness |
| Policy | `internal/core` | `Route`, `TierOf`, guardrails, session routing (`PlanForSession`, `RouteForSession`, `InferSessionFromCurrent`, `CanWriteSessionID`), self-check, `HarnessCapabilities`. Calls `classify`. Imports `quotamodel` |
| Quota types | `internal/quotamodel` | Pure evidence: `Status`, `Window`, `Snapshot`, `Validate`, `Evaluate`, `Abandoned`, `SourceName`. No file I/O |
| Session list (next cut, not landed) | `internal/session` | `SessionList` and the loaders that do not need `Decision` or `Resolver`: file, hook marks, discovered file, quota snapshot attachment. Must not import `core` |
| Shared services | `internal/policyfile`, `adapt`, `quota`, `telemetry`, `catalog`, `hookctx` | Optional behaviour plugged into core, state, I/O |
| Hook runner | `cmd/downshift` `runHookAdapter` | Reads the payload once, sets shared context, records telemetry, fails open |
| Adapters | `internal/adapters/<harness>` | Parse the payload, declare capabilities, emit the wire response |

Dependencies point inward: adapters may import core and shared services; core
imports `classify` and `quotamodel` and never imports `catalog`, `telemetry`,
`adapt`, `policyfile`, `quota` or an adapter (checked with
`go list -deps ./internal/core`). The next cut, not landed, moves the session
list to `internal/session`. `core` may import `session`; `session` must not
import `core`. Optional layers plug into core through a hook variable
(`core.MemoryAdjustHook`, `core.TierFloorHook`) registered in their `init`.
Debt status (2026-10-18):

- Configuration read once at the edge: done. `core.Config` (`internal/core/env.go`)
  types every `DOWNSHIFT_*` variable core reads; `runHookAdapter` calls
  `core.SetConfig(core.ConfigFromEnv(os.Getenv))` once per invocation and
  restores it on return, and offline commands and tests read the live
  environment through the same accessors (test:
  `TestHookRunnerReadsConfigOnce`).
- `hookctx`: core no longer imports it directly; it reaches core only through
  helper packages that stay in-process (`semantic` hash embedder).
- Classification vs route policy: done. `internal/classify` scores the prompt. `core.TierOf` and `Route` stay in policy. Session routing stays in policy with them.
- Quota evidence types: done. `internal/quotamodel` holds the pure quota
  evidence types (`Status`, `Window`, `Snapshot`, `Validate`, `Evaluate`,
  `Abandoned`, `SourceName`) and does no file I/O. `go list -deps
  ./internal/core` includes `quotamodel` and `classify` and does not include
  `internal/quota`. `SessionList.Usage` is a `*quotamodel.Snapshot`.
  `internal/quota` keeps `Load`, `Store`, `ParseNative` and native
  registration, and aliases those types so adapters can keep saying
  `quota.Snapshot`. File reads and native parsing enter through hooks
  registered by `quota` (`LoadQuotaHook`, `LoadNativeQuotaHook`,
  `ParseNativeQuotaHook`), the same pattern as `MemoryAdjustHook`.
- Session list: the next cut, not landed. `SessionList` and the loaders that
  do not need `Decision` or `Resolver` still live in `internal/core`: the
  operator file, hook marks, the discovered file, and quota snapshot
  attachment. A new package `internal/session` will own them.
  `PlanForSession`, `RouteForSession`, `InferSessionFromCurrent` and
  `CanWriteSessionID` stay in `internal/core` because they need `Decision`,
  `Tier`, `Model` and `Resolver`. `core` may import `session`. `session` must
  not import `core`.

## Before you write code

1. Name the behaviour in harness-free terms ("raise the tier to a floor"), not
   in terms of one harness's payload.
2. Find the layer it belongs to. If you are about to edit more than one adapter
   for the same logic, stop: the logic belongs below them.
3. If it needs data from the payload, add an optional method to each adapter's
   `Event` (for example `ProjectDir()`) and read it in the runner, not in core.
4. If it needs a per-harness fact (an env variable, a limit), add a field to
   `HarnessCapabilities` and read it through `core.AllHarnessCaps()`.
5. If a harness cannot apply the result (no rewrite path, plan restriction),
   the shared layer still computes it; record the limit in
   `docs/reference/harness-matrix.md` with evidence and a date.

## Before you open a change

- [ ] The same logic does not appear in two adapters.
- [ ] No shared tool (`doctor`, stats, evals) names a harness.
- [ ] A test goes through `runHookAdapter` or `core.Route`, not only one adapter.
- [ ] Offline commands (`benchmark`, `try`, evals) behave the same with and
      without project context.
- [ ] Missing or invalid optional input fails open.
- [ ] The harness matrix says what each harness can and cannot honor.

## Worked examples (2026-10-11)

- **Project tier floor.** First version called `ApplyFloor` from five adapters.
  Corrected to `core.TierFloorHook` (registered by `policyfile`), fed by
  `hookctx.ProjectDir`, which the runner sets from an optional `ProjectDir()`
  method. Test: `TestHookE2E_ProjectFloorThroughSharedRunner`.
- **Model-override warning.** First version hard-coded one harness's variable
  in `doctor`. Corrected to `HarnessCapabilities.ModelOverrideEnv`.
- **Audit skill.** First version assumed one harness's settings path. Corrected
  to point at the per-harness sections of `docs/guides/install.md`.
