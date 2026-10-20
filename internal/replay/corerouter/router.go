// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package corerouter answers the replay's routing questions with the real
// router: core.Route for the decision, then the same PlanSession and write
// guard the Claude Code adapter applies before it writes a model. A policy is a
// set of knobs that map onto core.Config, so an experiment changes exactly the
// switches the hook reads, nothing else.
//
// Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift
package corerouter

import (
	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/replay"
)

// Harness is the harness the replay decides for: the spawns are Claude Code's.
const Harness = "claude-code"

// Knob names a policy may set. Values are "1" to turn a switch on.
const (
	KnobNBDownshift     = "nb_downshift"
	KnobExplicitUpshift = "explicit_upshift"
)

// Router decides cases with core.
type Router struct {
	Res core.Resolver
	// Models is every model id the session may run on. The write guard only
	// rewrites to an id the session lists, so the replay lists the catalog's.
	Models []string
}

// Route returns the model the policy would write for the case, or a hold.
func (r Router) Route(c replay.Case, p replay.Policy) (replay.Verdict, error) {
	cfg := core.Config{
		NBDownshift:     p.Knobs[KnobNBDownshift] == "1",
		ExplicitUpshift: p.Knobs[KnobExplicitUpshift] == "1",
	}
	restore := core.SetConfig(cfg)
	defer restore()

	d := core.Route(c.Prompt, Harness, c.Baseline, r.Res)
	d.Baseline.RequestedID = c.Baseline
	session := core.SessionList{IDs: r.Models, Known: true}
	plan := d.PlanSession(core.ClaudeCodeCaps, r.Res, session, c.Baseline)

	switch {
	case plan.HoldForeign, plan.PreserveExplicit, plan.CreditHeld:
		return replay.Verdict{Held: true, Reasons: []string{holdReason(plan)}}, nil
	case !plan.RewriteModel:
		return replay.Verdict{Held: true, Reasons: append(reasons(d), "no_rewrite")}, nil
	case !core.CanWriteSessionID(Harness, plan.Model.ID, plan.Session, r.Res):
		return replay.Verdict{Held: true, Reasons: []string{"session_guard"}}, nil
	}
	return replay.Verdict{Model: plan.Model.ID, Reasons: reasons(d)}, nil
}

func holdReason(plan core.RewritePlan) string {
	switch {
	case plan.HoldForeign:
		return "foreign_model"
	case plan.PreserveExplicit:
		return "explicit_only"
	default:
		return "credit_held"
	}
}

// reasons lists the guardrail rules that held or corrected the decision.
func reasons(d core.Decision) []string {
	out := make([]string, 0, len(d.Corrections))
	out = append(out, d.Corrections...)
	return out
}
