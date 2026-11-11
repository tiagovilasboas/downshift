// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core

// ObserveSpawn computes the routing decision for a spawn the harness gives no
// way to change: no model field to rewrite and no model to ask a respawn with
// (Kiro IDE invoke_sub_agent, kiro-cli subagent). The decision is recorded for
// telemetry only. Its note says observation-only and names the tier a plan
// would pick, so `report` can show what routing would have done. Callers must
// not block or rewrite on it, and must return an empty note to the shared
// runner so the outcome is recorded as allow, not as an emitted rewrite.
func ObserveSpawn(req Request, caps HarnessCapabilities, session SessionList, res Resolver) Decision {
	res = ResolveSessionModels(req.Harness, session, res)
	decision := RouteSpawn(req, res)
	decision.Baseline.RequestedID = req.CurrentModelID
	plan := decision.PlanSession(caps, res, session, req.CurrentModelID)
	decision.Note = RoutingNote{
		Verdict: decision.Verdict.String(),
		Reason:  "observation-only",
		TierTo:  plan.Model.Tier.String(),
	}
	return decision
}
