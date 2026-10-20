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

func TestShadowRowIsNeverSelectedEvenWithExplicitUpshift(t *testing.T) {
	t.Setenv("DOWNSHIFT_EXPLICIT_UPSHIFT", "1")
	session := core.KnownSession([]string{"composer-2.5", shadowRowID})
	res := shadowAt{Resolver: cat, shadow: shadowRowID}

	plan := upshiftDecisionFor("composer-2.5").PlanForSession(core.CursorCaps, res, session)
	if plan.Model.ID == shadowRowID || (plan.RewriteModel && plan.Model.ID == shadowRowID) {
		t.Fatalf("plan = %+v, a shadow allowlist row must never be an upshift target", plan)
	}
	if core.CanWriteSessionID("cursor", shadowRowID, session, res) {
		t.Fatal("a shadow allowlist row must not be writable, explicit flag on")
	}
}

func TestExplicitRowStillSelectableWhenNotShadow(t *testing.T) {
	t.Setenv("DOWNSHIFT_EXPLICIT_UPSHIFT", "1")
	session := core.KnownSession([]string{"composer-2.5", shadowRowID})
	plan := upshiftDecisionFor("composer-2.5").PlanForSession(core.CursorCaps, cat, session)
	if !plan.RewriteModel || plan.Model.ID != shadowRowID {
		t.Fatalf("plan = %+v, the explicit_only row must stay selectable when it is not a shadow row", plan)
	}
}
