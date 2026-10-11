# Layering: every feature serves every harness

Owner rule, 2026-10-11. This is the checklist behind the short rule in
[AGENTS.md](../../AGENTS.md). Read it before adding or changing a feature,
adapter, hook, command or policy.

## Layers

| Layer | Where | Holds |
|---|---|---|
| Policy | `internal/core` | Classification, `Route`, tiers, risk, self-check, `HarnessCapabilities` |
| Shared services | `internal/policyfile`, `adapt`, `quota`, `telemetry`, `catalog`, `hookctx` | Optional behaviour plugged into core, state, I/O |
| Hook runner | `cmd/downshift` `runHookAdapter` | Reads the payload once, sets shared context, records telemetry, fails open |
| Adapters | `internal/adapters/<harness>` | Parse the payload, declare capabilities, emit the wire response |

Dependencies point inward: adapters may import core and shared services; core
imports none of them. Optional layers plug into core through a hook variable
(`core.MemoryAdjustHook`, `core.TierFloorHook`) registered in their `init`.

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
   `docs/harness-matrix.md` with evidence and a date.

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
  to point at the per-harness sections of `docs/install.md`.
