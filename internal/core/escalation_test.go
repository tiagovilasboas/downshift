// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core_test

import (
	"testing"

	"github.com/tiagovilasboas/downshift/internal/catalog"
	"github.com/tiagovilasboas/downshift/internal/core"
)

var escalCat = catalog.Load()

// --- Route produces correct Intent ---

func TestRoute_TrivialPrompt_TrivialIntent(t *testing.T) {
	d := core.Route("rename the userId variable", "claude-code", "", escalCat)
	if d.Intent != core.TrivialIntent {
		t.Errorf("trivial prompt intent = %s, want trivial", d.Intent)
	}
}

func TestRoute_NormalModelNotPreserved(t *testing.T) {
	frontierID := escalCat.ModelFor("codex", core.TierFrontier).ID
	d := core.Route("rename the variable", "codex", frontierID, escalCat)
	if d.Intent == core.PreservedIntent {
		t.Errorf("normal frontier model should not produce PreservedIntent")
	}
}

// --- EscalationIntent.String() ---

func TestEscalationIntent_String(t *testing.T) {
	cases := map[core.EscalationIntent]string{
		core.TrivialIntent:   "trivial",
		core.NormalIntent:    "normal",
		core.ReviewIntent:    "review",
		core.PreservedIntent: "preserved",
	}
	for intent, want := range cases {
		if got := intent.String(); got != want {
			t.Errorf("%T(%d).String() = %q, want %q", intent, intent, got, want)
		}
	}
}

// --- Summary includes intent suffix for review/preserved ---

func TestDecisionSummary_ReviewIntentSuffix(t *testing.T) {
	d := core.Route("do a code review of the auth module", "claude-code", "", escalCat)
	s := d.Summary()
	if d.Intent == core.ReviewIntent {
		found := false
		for _, sub := range []string{"[review]", "review"} {
			if len(s) >= len(sub) {
				for i := 0; i <= len(s)-len(sub); i++ {
					if s[i:i+len(sub)] == sub {
						found = true
						break
					}
				}
			}
		}
		if !found {
			t.Errorf("Summary() for ReviewIntent missing 'review' suffix: %q", s)
		}
	}
}
