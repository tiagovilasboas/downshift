// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core_test

import (
	"testing"

	"github.com/tiagovilasboas/downshift/internal/core"
)

// shadowAt marks one id as an allowlist shadow row, the way the catalog does
// for an allowlisted gateway row.
type shadowAt struct {
	core.Resolver
	shadow string
}

func (s shadowAt) IsShadow(_, id string) bool { return id == s.shadow }

const shadowRowID = "claude-fable-5-1-thinking-high"

func upshiftDecisionFor(current string) core.Decision {
	return core.Decision{
		Harness:   "cursor",
		Verdict:   core.VerdictUpshift,
		Confident: true,
		Tier:      core.TierFrontier,
		Model:     core.Model{ID: current, Harness: "cursor"},
		Baseline: core.Baseline{
			RequestedID:  current,
			CurrentModel: core.Model{ID: current, Harness: "cursor"},
		},
	}
}

func TestShadowRowIsNeverSelectedOnUpshift(t *testing.T) {
	session := core.KnownSession([]string{"composer-2.5", shadowRowID})
	res := shadowAt{Resolver: cat, shadow: shadowRowID}

	plan := upshiftDecisionFor("composer-2.5").PlanForSession(core.CursorCaps, res, session)
	if plan.Model.ID == shadowRowID || (plan.RewriteModel && plan.Model.ID == shadowRowID) {
		t.Fatalf("plan = %+v, a shadow allowlist row must never be an upshift target", plan)
	}
	if core.CanWriteSessionID("cursor", shadowRowID, session, res) {
		t.Fatal("a shadow allowlist row must not be writable")
	}
}

// An explicit_only row (here fable) is never an upshift target even when it is
// not a shadow row: routing never selects the owner's deliberate choice. The
// legacy explicit-upshift opt-in that once let it was removed.
func TestExplicitRowNeverSelectableOnUpshift(t *testing.T) {
	session := core.KnownSession([]string{"composer-2.5", shadowRowID})
	plan := upshiftDecisionFor("composer-2.5").PlanForSession(core.CursorCaps, cat, session)
	if plan.RewriteModel && plan.Model.ID == shadowRowID {
		t.Fatalf("plan = %+v, the explicit_only row must never be selected", plan)
	}
	if core.CanWriteSessionID("cursor", shadowRowID, session, cat) {
		t.Fatal("an explicit_only row must never be writable")
	}
}
