// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package decide

import (
	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/routing/contract"
)

// raiseToFloor applies the project tier floor after the guard. The floor only
// raises: a decision already at or above it is untouched, and it also lifts a
// decision the guard held (never_widen included), since it is a requirement of
// the project, not a downshift. An unknown tier, or a floor tier that resolves
// to no automatic model, leaves the decision as it is (doubt keeps).
func raiseToFloor(in Input, d core.Decision, n contract.Note) (core.Decision, contract.Note) {
	floor, ok := in.Floor.Floor(in.Class, in.Risk)
	if !ok || in.Catalog == nil || rank(n.TierTo) < 0 {
		return d, n
	}
	want := contract.TierOf(floor)
	if rank(want) <= rank(n.TierTo) {
		return d, n
	}
	target := targetModel(in, want)
	harness := string(in.Request.Harness)
	if target.ID == "" || in.Catalog.IsExplicitOnly(harness, target.ID) || !in.Catalog.IsExactID(harness, target.ID) {
		return d, n
	}
	d.Tier, d.Model, d.Effort = target.Tier, target, core.EffortFor(target.Tier)
	d.Guards.PolicyFloor = true
	d.Verdict, d.SafeVerdict = core.VerdictUpshift, core.VerdictUpshift
	d.Reason = ""
	if cur := d.Baseline.CurrentModel; cur.ID != "" && cur.Tier >= target.Tier {
		d.Verdict, d.SafeVerdict = core.VerdictOK, core.VerdictOK
		if cur.Tier > target.Tier {
			d.Verdict, d.SafeVerdict = core.VerdictDownshift, core.VerdictDownshift
		}
	}
	n.Verdict, n.Reason, n.TierTo = contract.VerdictGuardFloor, contract.ReasonNone, want
	return d, n
}
