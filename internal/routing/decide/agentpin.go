// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package decide

import (
	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/routing/contract"
)

// noteAboveAgentDefault flags a spawn whose call named a model above the tier
// its agent definition declares. It never changes the model. The reason code is
// recorded when the spawn stayed as pinned (Guards.AboveAgentDefault); the note
// for the orchestrator is set only where the harness can return one.
func noteAboveAgentDefault(in Input, d core.Decision, n contract.Note) core.Decision {
	req := in.Request
	t := in.AgentDefaultTier
	if req.ModelInherited || req.RequestedModel == "" || !t.Valid() || !n.TierFrom.Valid() || rank(t) >= rank(n.TierFrom) {
		return d
	}
	// Recorded only when the spawn stayed as pinned, so the code's meaning
	// ("nothing moved") is true: a floor, rule or hint that moved it is not this.
	if n.TierTo != n.TierFrom {
		return d
	}
	d.Guards.AboveAgentDefault = true
	caps, _ := core.CapsFor(string(req.Harness))
	if !caps.CanReturnNote || d.Advice != "" {
		return d
	}
	pinned, _ := n.TierFrom.Core()
	deflt, _ := t.Core()
	if in.AgentName == "" {
		return d
	}
	d.Advice = core.AdviceAboveAgentDefault
	d.AgentPin = core.AgentPin{Agent: in.AgentName, Default: deflt, Pinned: pinned}
	return d
}
