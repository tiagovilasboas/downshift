<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# T1: one `deterministicLift` helper; tier floor not held; drop dead gate

**Problem.** The set "a deterministic rule lifted this tier" (risk floor, graph
escalation, opted-in NB upshift, operator tier floor) is copied in R7, R8, R9,
`ShouldRewriteModel` and telemetry `frontierOnDoubt`, and the copies differ.
R7 and R8 ignore `PolicyFloor`, so `.downshift/policy.json` is silently held for
every unconfident prompt. Reproduction: a `TierFloorHook` calling
`d.RaiseTier(core.TierFrontier, ...)`, prompt `"help"`: with a haiku parent R8
holds it; with no current model R7 holds it.

**Also.** `Decision.ShouldRewriteModel()` has no production caller (only tests);
it reads like the gate but is not. Delete it and its comments, keeping the
behaviour in `Check`. Update tests that call it to assert on `Check`/`Safe`.

**Done when.**
- `deterministicLift(d Decision) bool` in `internal/core/selfcheck.go`
  (RiskFloor || GraphEscalated || NBUpshift.Applied || PolicyFloor) used by R7,
  R8, R9.
- E2E test through `core.Route` + `PlanSession` with a tier floor and an
  unconfident prompt, haiku parent and unknown parent: the rewrite happens.
- `ShouldRewriteModel` removed; truth-table test in
  `cost_safety_invariants_test.go` re-expressed on `Check`.
- telemetry `frontierOnDoubt` keeps using reason codes (TIER_FLOOR already
  counts).
- `go test -p 2 ./...` green.
