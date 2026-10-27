# Layering: every feature serves every harness

Owner rule, 2026-10-11. This is the checklist behind the short rule in
[AGENTS.md](../../AGENTS.md). Read it before adding or changing a feature,
adapter, hook, command or policy.

## Layers

| Layer | Where | Holds |
|---|---|---|
| Classification | `internal/classify` | Regex class, signals, language guard, risk floor. No tier and no harness |
| Policy | `internal/core` | `Route`, `TierOf`, guardrails, session routing (`PlanForSession`, `RouteForSession`, `CatalogSessionFromCurrent`, `CanWriteSessionID`), self-check, `HarnessCapabilities`. Calls `classify`. Imports `fuse`, `quotamodel` and `session`. Score selection stays here: `ScoreHook` is the port, and `core` does not import `scoreindex` |
| Quota types | `internal/quotamodel` | Pure evidence: `Status`, `Window`, `Snapshot`, `Validate`, `Evaluate`, `Abandoned`, `SourceName`. No file I/O |
| Session list | `internal/session` | `SessionList` and the loaders that do not need `Decision` or `Resolver`: file, hook marks, discovered file, quota snapshot attachment. Landed: `core` imports `session`; `session` must not import `core` |
| Semantic boost policy | `internal/fuse` | Landed. Raise-only boost, including the no-signal skip. Imports `classify` and `semantic`. Does not import `core`. `core` keeps `ClassifyWithSemantic` as a wrapper |
| NB opinion hooks | `internal/nbopinion` | Landed (`4d3e857`, PR 156). Hook funcs that call `nbtier.Suggest` and `nbtier.Trivial`. May import `core` and `nbtier`. Core production sources do not import `nbopinion` or `nbtier` (core test files import `catalog`, `nbopinion`, `nbtier` and `quota`; they are not in the binary). `NBUpshift` and `NBDownshift` stay on `Decision` in `core`. `cmd` blank-imports `nbopinion` so `init` runs. The opinion stays in-process and does not exec |
| Shared services | `internal/policyfile`, `adapt`, `routeadapt`, `scoreindex`, `quota`, `telemetry`, `catalog`, `discovery`, `models`, `hookctx` | Optional behaviour plugged into core, state, I/O. All import `core`; `core` imports none of them |
| Hook runner | `cmd/downshift` `runHookAdapter` | Reads the payload once, sets shared context, records telemetry, fails open |
| Adapters | `internal/adapters/<harness>` | Parse the payload, declare capabilities, emit the wire response |

Dependencies point inward: adapters may import core and shared services; core
imports `classify`, `fuse`, `quotamodel` and `session`. Core production
sources never import `quota`, `nbtier`, `nbopinion`, `catalog`, `telemetry`,
`adapt`, `policyfile`, `scoreindex` or an adapter (checked with `go list
-deps ./internal/core`; re-verified 2026-10-19). Core test files import `catalog`,
`nbopinion`, `nbtier` and `quota`; they are not in the binary. `session` must not import `core`. `fuse` imports
`classify` and `semantic` and does not import `core`. `PlanForSession`,
`RouteForSession`, `CatalogSessionFromCurrent` and `CanWriteSessionID` stay in
`core`. Score selection stays in `core` on purpose: `ScoreHook` is already
the port, and `core` does not import `scoreindex`. Do not propose a package
that merges score selection with the naive-Bayes opinion. The naive-Bayes
hooks are landed (`4d3e857`, PR 156): `internal/nbopinion` calls
`nbtier.Suggest` and `nbtier.Trivial`, in-process, with no exec. Optional
layers plug into core through a hook variable (`core.MemoryAdjustHook`,
`core.TierFloorHook`) registered in their `init`. Debt status (2026-10-19):

- Configuration read once at the edge: done. `core.Config` (`internal/core/env.go`)
  types every `DOWNSHIFT_*` variable core reads; `runHookAdapter` calls
  `core.SetConfig(core.ConfigFromEnv(os.Getenv))` once per invocation and
  restores it on return, and offline commands and tests read the live
  environment through the same accessors (test:
  `TestHookRunnerReadsConfigOnce`). Env stays this typed snapshot set at the
  edge (`SetConfig`). The landed NB cut left `env.go` there.
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
- Session list: done (`5f94e07`, PR 152). `internal/session` owns `SessionList`
  and the loaders that do not need `Decision` or `Resolver`: the operator
  file, hook marks, the discovered file, and quota snapshot attachment.
  `go list -deps ./internal/core` includes `session`. `go list -deps
  ./internal/session` does not include `core`. `PlanForSession`,
  `RouteForSession`, `CatalogSessionFromCurrent` and `CanWriteSessionID` stay
  in `internal/core` because they need `Decision`, `Tier`, `Model` and
  `Resolver`.
- Semantic boost policy: done (`110ef31`, PR 154). `internal/fuse` owns the
  raise-only boost, including the no-signal skip. It imports `classify` and
  `semantic`. `go list -deps ./internal/fuse` does not include `core`.
  `core` keeps `ClassifyWithSemantic` as a wrapper.
- Score selection stays in `core` (`score_select.go`) on purpose. `ScoreHook`
  is already the port. `go list -deps ./internal/core` does not include
  `scoreindex`. Do not propose a package that merges score selection with
  the naive-Bayes opinion.
- Naive-Bayes opinion hooks: done (`4d3e857`, PR 156). `internal/nbopinion`
  registers hook funcs that call `nbtier.Suggest` and `nbtier.Trivial`. It
  may import `core` and `nbtier`. `go list -deps ./internal/core` does not
  include `nbopinion` or `nbtier`. Core test files import `catalog`, `nbopinion`,
  `nbtier` and `quota`; they are not in the binary. `NBUpshift` and `NBDownshift` stay on
  `Decision` in `core`. `cmd` blank-imports `nbopinion` so `init` runs. The
  opinion stays in-process and does not exec. Env stays a typed snapshot
  set at the edge (`SetConfig`).

- Not enforced by a test (open gap): the import rules above are checked by hand
  with `go list -deps`. Registration by `init` also means `core.Route` depends
  on blank imports in `cmd/downshift` (`nbopinion`, `policyfile`, `quota`,
  `routeadapt`); adapter tests do not load `policyfile`, `nbopinion` or
  `routeadapt`. See "Known gaps and limits" in
  [architecture.md](../design/architecture.md).

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
