// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core_test

import (
	"testing"

	"github.com/tiagovilasboas/downshift/internal/core"
)

// TestSetDecider_RouteAsksTheInstalledDeciderAndRestores pins the seam: Route
// and RouteAgent hand every field to the active Decider, and the restore
// function puts the default back.
func TestSetDecider_RouteAsksTheInstalledDeciderAndRestores(t *testing.T) {
	const prompt = "rename the userId variable"
	before := core.Route(prompt, "claude-code", "")

	var got core.Request
	restore := core.SetDecider(core.DeciderFunc(func(req core.Request) core.Decision {
		got = req
		return core.Decision{Harness: req.Harness, Tier: core.TierFrontier, Checked: true}
	}))
	d := core.RouteAgent(prompt, "codex", "explorer", "cur-id", nil)
	if d.Tier != core.TierFrontier || d.Harness != "codex" {
		t.Fatalf("RouteAgent did not return the stub decision: %+v", d)
	}
	want := core.Request{Prompt: prompt, Harness: "codex", AgentType: "explorer", CurrentModelID: "cur-id"}
	if got != want {
		t.Fatalf("request = %+v, want %+v", got, want)
	}
	restore()

	after := core.Route(prompt, "claude-code", "")
	if after.Tier != before.Tier || after.Verdict != before.Verdict || after.Complexity != before.Complexity {
		t.Fatalf("restore did not reinstate the default decider: before %+v, after %+v", before, after)
	}
}

// TestSetDecider_NilMeansDefault keeps a nil Decider from panicking Route.
func TestSetDecider_NilMeansDefault(t *testing.T) {
	want := core.Route("rename the userId variable", "codex", "")
	defer core.SetDecider(nil)()
	if got := core.Route("rename the userId variable", "codex", ""); got.Tier != want.Tier || !got.Checked {
		t.Fatalf("nil decider: got %+v, want the default decision %+v", got, want)
	}
}

// TestDefaultDecider_KeepsTheRequestedModel pins the decider used where none
// is installed: the spawn runs on the model it asked for, and a model the
// catalog does not know is kept without a target (safe verdict OK, so the
// channel plan has nothing to write).
func TestDefaultDecider_KeepsTheRequestedModel(t *testing.T) {
	defer core.SetDecider(nil)()
	frontier := cat.ModelFor("claude-code", core.TierFrontier)
	cases := []struct {
		name      string
		current   string
		res       core.Resolver
		verdict   core.Verdict
		wantID    string
		confident bool
	}{
		{"known model", frontier.ID, cat, core.VerdictOK, frontier.ID, true},
		{"unknown model", "no-such-model", cat, core.VerdictUnknown, "", false},
		{"no model", "", cat, core.VerdictUnknown, "", false},
		{"no resolver", frontier.ID, nil, core.VerdictUnknown, "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var d core.Decision
			if tc.res == nil {
				d = core.Route("rename the variable", "claude-code", tc.current)
			} else {
				d = core.Route("rename the variable", "claude-code", tc.current, tc.res)
			}
			if d.Verdict != tc.verdict || d.SafeVerdict != core.VerdictOK || d.Model.ID != tc.wantID || d.Confident != tc.confident {
				t.Fatalf("verdict %v safe %v model %q confident %v, want %v OK %q %v",
					d.Verdict, d.SafeVerdict, d.Model.ID, d.Confident, tc.verdict, tc.wantID, tc.confident)
			}
		})
	}
}
