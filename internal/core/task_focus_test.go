// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core_test

import (
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/core"
)

// focusBrief is a long, unbounded parent brief: a background paragraph that
// names hard topics, then an explicit task.
func focusBrief(background, task string) string {
	return "Background: The repository uses Go 1.27 and a small set of vetted dependencies. The CI pipeline runs on " +
		"every change and publishes coverage. The maintainers prefer small focused commits with clear messages in English, " +
		"and every package carries a doc comment that explains its purpose. Several people work in this repository at the same " +
		"time, each in a separate checkout, so keep the change small and easy to review. " +
		background + "\n\nTask: " + task
}

// A mechanical task inside a long brief whose context names the architecture
// and a consensus protocol is downshifted end to end, and the trace says why.
func TestRoute_TaskFocusDownshiftsMechanicalTask(t *testing.T) {
	p := focusBrief("The architecture has a classifier, a policy layer and adapters, and the store uses a consensus protocol.",
		"rename the variable cnt to count in internal/telemetry/summary.go.")
	if n := len(strings.Fields(p)); n < 80 {
		t.Fatalf("precondition: %d words", n)
	}
	d := core.Route(p, "claude-code", catID("claude-code", core.TierFrontier), cat)
	if d.Complexity != core.Trivial || !d.Confident || !d.Guards.TaskFocus {
		t.Fatalf("got %v confident=%v focus=%v, want confident TRIVIAL by task focus", d.Complexity, d.Confident, d.Guards.TaskFocus)
	}
	if !checkWrites(d) || d.Tier != core.TierSmall {
		t.Fatalf("want an applied downshift to small, got tier=%v safe=%v corrections=%v", d.Tier, d.SafeVerdict, d.Corrections)
	}
}

// Risk words in the context still floor the task: no write below mid.
func TestRoute_TaskFocusKeepsRiskFloor(t *testing.T) {
	for _, bg := range []string{
		"The auth service issues JWTs and stores refresh tokens; the architecture is a monolith.",
		"This module handles refunds and payment processing; its architecture is old.",
		"The config struct also holds the API keys and the database password; the architecture is a monolith.",
	} {
		p := focusBrief(bg, "rename the helper fmtVal to formatValue in internal/app/values.go.")
		if n := len(strings.Fields(p)); n < 80 {
			t.Fatalf("precondition: %d words", n)
		}
		d := core.Route(p, "claude-code", catID("claude-code", core.TierFrontier), cat)
		if len(d.Risk) == 0 || !d.Guards.RiskFloor {
			t.Fatalf("precondition: %q should floor on risk, got risk=%v floor=%v", bg, d.Risk, d.Guards.RiskFloor)
		}
		if d.Tier < core.TierMid || (checkWrites(d) && d.Tier != core.TierFrontier) {
			t.Errorf("risky brief %q: tier=%v writes=%v corrections=%v; want no write below the requested model", bg, d.Tier, checkWrites(d), d.Corrections)
		}
	}
}

// A simple task next to a hard-topic context is held, not downshifted, and
// never upshifted: doubt does not spend money in either direction.
func TestRoute_TaskFocusHoldsOnDoubt(t *testing.T) {
	p := focusBrief("The architecture is a set of services behind one gateway.",
		"add a verbose flag to the stats command in cmd/tool/stats.go.")
	if n := len(strings.Fields(p)); n < 80 {
		t.Fatalf("precondition: %d words", n)
	}
	for _, tier := range []core.Tier{core.TierSmall, core.TierMid, core.TierFrontier} {
		current := catID("claude-code", tier)
		d := core.Route(p, "claude-code", current, cat)
		if d.Confident {
			t.Fatalf("got confident %v, want an unconfident read", d.Complexity)
		}
		if checkWrites(d) && d.Model.ID != current {
			t.Errorf("current %v: model rewritten to %s on doubt (corrections=%v)", tier, d.Model.ID, d.Corrections)
		}
	}
}
