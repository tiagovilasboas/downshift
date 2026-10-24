// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core_test

import (
	"reflect"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/core"
)

const (
	unsureText  = "look around the repo and tell me how the router is wired"
	complexText = "design the architecture for a multi-region failover system with consistency tradeoffs"
	riskyText   = "find where the auth token is stored and how passwords are hashed"
	hintHarness = "claude-code"
)

func TestRouteAgent_TypeHint(t *testing.T) {
	opus := catID(hintHarness, core.TierFrontier)
	haiku := catID(hintHarness, core.TierSmall)
	cases := []struct {
		name      string
		prompt    string
		agentType string
		current   string
		wantTier  core.Tier
		wantHint  bool
		wantWrite bool
	}{
		{"Explore + unsure text -> small, confident, written", unsureText, "Explore", opus, core.TierSmall, true, true},
		{"Plan + unsure text -> mid", unsureText, "Plan", opus, core.TierMid, true, true},
		{"Explore + confident COMPLEX text -> classifier wins", complexText, "Explore", opus, core.TierFrontier, false, false},
		{"Plan on a small session -> capped at session, no upshift", unsureText, "Plan", haiku, core.TierSmall, true, false},
		{"Explore + risk text -> risk floor holds, no hint", riskyText, "Explore", opus, core.TierMid, false, false},
		{"unknown type -> unchanged", unsureText, "my-custom-agent", opus, core.TierMid, false, false},
		{"empty type -> unchanged", unsureText, "", opus, core.TierMid, false, false},
		{"pinned built-in has no hint", unsureText, "claude-code-guide", opus, core.TierMid, false, false},
		{"unknown current model -> hint not applied", unsureText, "Explore", "", core.TierMid, false, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			d := core.RouteAgent(c.prompt, hintHarness, c.agentType, c.current, cat)
			if d.Tier != c.wantTier || d.Guards.AgentTypeHint != c.wantHint {
				t.Fatalf("tier=%v hint=%v, want tier=%v hint=%v", d.Tier, d.Guards.AgentTypeHint, c.wantTier, c.wantHint)
			}
			if c.wantHint && !d.Confident {
				t.Error("a hinted decision is confident")
			}
			if got := d.Verdict == core.VerdictUpshift; got {
				t.Error("the hint must never produce an upshift")
			}
			if got := checkWrites(d); got != c.wantWrite {
				t.Errorf("writes=%v, want %v (verdict=%v corrections=%v)", got, c.wantWrite, d.Verdict, d.Corrections)
			}
		})
	}
}

// Without a type, RouteAgent is Route: untyped routing (and so the labelled
// benchmark, which classifies text only) cannot change.
func TestRouteAgent_UntypedEqualsRoute(t *testing.T) {
	for _, p := range []string{unsureText, complexText, riskyText} {
		a := core.Route(p, hintHarness, catID(hintHarness, core.TierFrontier), cat)
		b := core.RouteAgent(p, hintHarness, "", catID(hintHarness, core.TierFrontier), cat)
		if !reflect.DeepEqual(a, b) {
			t.Errorf("Route and untyped RouteAgent differ for %q", p)
		}
	}
}

// Hints are data: a harness with none ignores the type entirely.
func TestRouteAgent_HarnessWithoutHintsIgnoresType(t *testing.T) {
	for _, n := range core.AllHarnessCaps() {
		if len(n.Caps.AgentTypeHints) != 0 && n.Harness != "claude-code" {
			t.Errorf("%s declares hints without evidence", n.Harness)
		}
	}
	cur := catID("codex", core.TierFrontier)
	a := core.Route(unsureText, "codex", cur, cat)
	b := core.RouteAgent(unsureText, "codex", "Explore", cur, cat)
	if !reflect.DeepEqual(a, b) {
		t.Error("codex has no hints; the type must not change the decision")
	}
}
