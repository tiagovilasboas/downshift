<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# Design note: a `core.Engine` with explicit ports (Fullstack P1-1)

Downshift by Tiago Vilas Boas
https://github.com/tiagovilasboas/downshift

Status: proposal, 2026-10-19. Nothing here is implemented.

## Problem

`core.Route(prompt, harness, current, resolver...)` is a pure-looking function
whose result depends on package-level variables that other packages fill in
from `init()` (blank imports in `cmd/downshift/main.go`):

| Hook (in `internal/core`) | Set by | Effect when unset |
|---|---|---|
| `NBSuggestHook`, `NBTrivialHook` | `nbopinion` | no naive-Bayes opinion |
| `TierFloorHook` | `policyfile` | no project tier floor |
| `MemoryAdjustHook` | `routeadapt` | no adapt-memory adjustment |
| `LoadQuotaHook`, `LoadNativeQuotaHook`, `ParseNativeQuotaHook`, `RegisterNativeQuotaHook` | `quota` | quota unknown |

Consequences, all observed in the code and tests:

1. **The same call returns different decisions in different binaries or test
   packages.** `core_test` links what it imports; the claudecode adapter tests
   blank-import only `policyfile` (`claudecode_test.go`), so they run without
   the NB opinion, the quota gate and adapt memory. A passing adapter test says
   nothing about the production tier floor interplay.
2. **A missing blank import fails silently**, by design (fail-open): the
   router just routes with less information. Nothing in CI notices.
3. **Tests that need a hook mutate globals** (`TierFloorHook` in
   `tier_floor_lift_test.go`) and must restore them, which blocks `t.Parallel`
   and invites leaks between tests.
4. `core.SetConfig` already shows the better shape (a value set once at the
   edge by `runHookAdapter`, restored on return), but the hooks do not follow it.

## Why the hooks exist

The dependency rule: core production code must not import `quota`, `nbtier`,
`catalog`, `telemetry`, adapters (enforced by `internal/archtest`). The hooks
are the inversion: core owns the *shape*, the outer package supplies the
*behaviour*. That part is right and should stay. What is wrong is that the
supply is ambient (package state set in `init`) instead of explicit.

## Minimal proposal

Keep the hook function types. Group them in one struct that core defines, and
pass it explicitly:

```go
// package core
type Ports struct {
    NBSuggest      func(prompt string, prodTier int) (bool, int, float64)
    NBTrivial      func(prompt string) (bool, float64)
    TierFloor      func(d *Decision, currentModelID string, r Resolver)
    MemoryAdjust   func(prompt string, d *Decision, res Resolver)
    Quota          QuotaPort   // Load, LoadNative, ParseNative, RegisterNative
}

type Engine struct { ports Ports; cfg Config }

func NewEngine(p Ports, c Config) *Engine
func (e *Engine) Route(prompt, harness, current string, r Resolver) Decision
```

- `core.Route` stays as a thin wrapper over a package-default `Engine` built
  from today's hook variables, so every existing caller and test compiles
  unchanged. This is the migration seam; nothing is big-bang.
- `cmd/downshift` builds one `Ports` value from the real packages in one place
  (replacing the blank imports) and gives `runHookAdapter` an `*Engine`.
  Adapters receive the engine through the existing `handle` closure instead of
  calling the package function.
- Tests construct `NewEngine(core.Ports{...})` with only the ports they need,
  with no globals and `t.Parallel` allowed. A shared `testengine.Production()`
  helper builds the full production wiring so adapter tests can run "as shipped".
- A conformance test in `cmd/downshift` asserts that the production `Ports`
  has no nil field (the silent-missing-import failure becomes a test failure).

## Steps, each shippable alone

1. Add `Ports` and `Engine`; implement `Route` on top; default engine reads the
   current hook variables. No behaviour change; run the stability golden
   (`TestClassifierStability`) and the corpus tests as the before/after proof.
2. Add the non-nil production-wiring test and `testengine.Production()`; switch
   the adapter tests that today blank-import one package to it. Expect new
   failures to show up here: that is the finding, and each one is a real
   interaction (floor vs NB, quota gate) the tests never exercised.
3. Pass the engine through `runHookAdapter`; convert `cmd/downshift` callers.
4. Delete the hook variables and the `init` registrations once nothing writes
   them; `internal/archtest` keeps forbidding the reverse imports.

## Not proposed

- No interface-per-port zoo: function fields keep the current signatures and
  cost nothing.
- No change to what the router decides. Any change in the stability golden
  during these steps is a bug in the refactor.

## Risks and unknowns

- The adapter tests that gain the full wiring may fail for reasons unrelated to
  the refactor (they were never run with the NB opinion). Budget for fixing or
  re-baselining them, with a before/after on the corpus.
- `Route` is variadic in its resolver today (`r ...Resolver`); the wrapper keeps
  that, the engine method takes a plain `Resolver`.
- Not measured: how many tests mutate hooks today (count by `git grep` before
  step 2) and the cost of constructing an `Engine` per hook process (expected
  to be a handful of assignments).
