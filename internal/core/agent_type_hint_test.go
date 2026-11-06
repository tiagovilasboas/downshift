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
