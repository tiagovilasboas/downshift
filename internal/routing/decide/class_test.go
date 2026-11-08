// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package decide

import (
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/core"
	"github.com/tiagovilasboas/downshift-ai/internal/routing/contract"
	"github.com/tiagovilasboas/downshift-ai/internal/routing/policy"
)

// classInput is a spawn that asked for the frontier model, with no profile,
// rule or hint, so only precedence 5 can move it.
func classInput(class string, decisive bool, risk []string, pol *policy.Policy) Input {
	return Input{
		Request:  req(func(*contract.Request) {}),
		Policy:   pol,
		Now:      now,
		Catalog:  newCatalog(),
		Class:    class,
		Decisive: decisive,
		Risk:     risk,
	}
}

func classOn() *policy.Policy { p := testPolicy(); p.ByClass = true; return p }

func TestDecideByClass(t *testing.T) {
	cases := []struct {
		name     string
		in       Input
		verdict  contract.Verdict
		modelID  string
		coreVerd core.Verdict
	}{
		{"decisive TRIVIAL moves frontier down to small", classInput("TRIVIAL", true, nil, classOn()),
			contract.VerdictClass, "small-model", core.VerdictDownshift},
		{"decisive SIMPLE moves frontier down to small", classInput("SIMPLE", true, nil, classOn()),
			contract.VerdictClass, "small-model", core.VerdictDownshift},
		{"a nil policy is the default policy: class routing is on", classInput("TRIVIAL", true, nil, nil),
			contract.VerdictClass, "small-model", core.VerdictDownshift},
		{"MEDIUM never moves", classInput("MEDIUM", true, nil, classOn()),
			contract.VerdictKept, "frontier-model", core.VerdictOK},
		{"COMPLEX never moves", classInput("COMPLEX", true, nil, classOn()),
			contract.VerdictKept, "frontier-model", core.VerdictOK},
		{"an undecided class keeps (doubt keeps)", classInput("TRIVIAL", false, nil, classOn()),
			contract.VerdictKept, "frontier-model", core.VerdictOK},
		{"a risk category keeps", classInput("TRIVIAL", true, []string{"secret"}, classOn()),
			contract.VerdictKept, "frontier-model", core.VerdictOK},
		{"routing.by_class false keeps", classInput("TRIVIAL", true, nil, testPolicy()),
			contract.VerdictKept, "frontier-model", core.VerdictOK},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			d, n := Decide(c.in)
			if n.Verdict != c.verdict || d.Model.ID != c.modelID || d.Verdict != c.coreVerd {
				t.Fatalf("verdict %q model %q core %q, want %q %q %q", n.Verdict, d.Model.ID, d.Verdict, c.verdict, c.modelID, c.coreVerd)
			}
		})
	}
}

// The class only moves down: a small request stays small, and an unknown
// requested model is kept.
func TestDecideByClassNeverRaisesNorGuesses(t *testing.T) {
	in := classInput("TRIVIAL", true, nil, classOn())
	in.Request.RequestedModel = "small-model"
	if d, n := Decide(in); n.Verdict != contract.VerdictKept || d.Model.ID != "small-model" {
		t.Fatalf("small request: verdict %q model %q, want kept small-model", n.Verdict, d.Model.ID)
	}
	in.Request.RequestedModel = "not-in-catalog"
	if d, n := Decide(in); n.Verdict != contract.VerdictKept || d.Model.ID != "" {
		t.Fatalf("unknown request: verdict %q model %q, want kept and no model", n.Verdict, d.Model.ID)
	}
}

// Every earlier step wins over the class: a pin, a profile, a rule and a hint.
func TestDecideByClassComesAfterDeclaredSteps(t *testing.T) {
	pinned := classInput("TRIVIAL", true, nil, classOn())
	pinned.Request.Pinned = true
	if _, n := Decide(pinned); n.Verdict != contract.VerdictKept {
		t.Errorf("pinned: verdict %q, want kept", n.Verdict)
	}
	profile := classInput("TRIVIAL", true, nil, classOn())
	profile.Request.Profile = "reviewer"
	if _, n := Decide(profile); n.Verdict != contract.VerdictProfile || n.TierTo != contract.TierMid {
		t.Errorf("profile: verdict %q tier %q, want profile mid", n.Verdict, n.TierTo)
	}
	hinted := classInput("TRIVIAL", true, nil, classOn())
	hinted.Request.Hint.Tier = contract.TierMid
	if _, n := Decide(hinted); n.Verdict != contract.VerdictHintDown || n.TierTo != contract.TierMid {
		t.Errorf("hint: verdict %q tier %q, want hint_down mid", n.Verdict, n.TierTo)
	}
}

// The project tier floor still runs last and raises a class move back up.
func TestDecideByClassRespectsTheFloor(t *testing.T) {
	in := classInput("TRIVIAL", true, nil, classOn())
	in.Floor = &policy.FloorPolicy{MinTier: "mid"}
	if d, _ := Decide(in); d.Model.ID != "mid-model" {
		t.Fatalf("floor mid: model %q, want mid-model", d.Model.ID)
	}
}

// The Decision carries the class the caller classified, never the zero value.
func TestDecideRecordsTheClass(t *testing.T) {
	for class, want := range map[string]core.Complexity{
		"TRIVIAL": core.Trivial, "SIMPLE": core.Simple, "MEDIUM": core.Medium, "COMPLEX": core.Complex, "": core.Medium,
	} {
		if d, _ := Decide(classInput(class, false, nil, classOn())); d.Complexity != want {
			t.Errorf("class %q: Complexity = %v, want %v", class, d.Complexity, want)
		}
	}
}

// A: a spawn Downshift moved down in this session just failed, so the class
// does not move the next one down; it says why.
func TestDecideByClassHoldsAfterAFailedDownshift(t *testing.T) {
	in := classInput("TRIVIAL", true, nil, classOn())
	in.RecentFailure = true
	d, n := Decide(in)
	if n.Verdict != contract.VerdictKept || n.Reason != contract.ReasonHeldAfterFailure || d.Model.ID != "frontier-model" {
		t.Fatalf("verdict %q reason %q model %q, want kept held_after_failure frontier-model", n.Verdict, n.Reason, d.Model.ID)
	}
	// A declared step still decides: the failure hold is the class step's only.
	in.Request.Hint.Tier = contract.TierMid
	if _, n := Decide(in); n.Verdict != contract.VerdictHintDown {
		t.Fatalf("hint after a failure: verdict %q, want hint_down", n.Verdict)
	}
}

func smallRequest(class string, pol *policy.Policy) Input {
	in := classInput(class, true, nil, pol)
	in.Request.RequestedModel = "small-model"
	return in
}

// B: a decisive COMPLEX task asked for on small runs as asked and carries the
// advice; nothing else does.
func TestDecideAdvisesComplexOnSmall(t *testing.T) {
	d, n := Decide(smallRequest("COMPLEX", classOn()))
	if d.Model.ID != "small-model" || n.Verdict != contract.VerdictKept || d.Advice != core.AdviceComplexOnSmall {
		t.Fatalf("model %q verdict %q advice %q, want small-model kept %q", d.Model.ID, n.Verdict, d.Advice, core.AdviceComplexOnSmall)
	}
	for name, in := range map[string]Input{
		"MEDIUM on small":      smallRequest("MEDIUM", classOn()),
		"COMPLEX on frontier":  classInput("COMPLEX", true, nil, classOn()),
		"COMPLEX not decisive": func() Input { i := smallRequest("COMPLEX", classOn()); i.Decisive = false; return i }(),
		"COMPLEX pinned":       func() Input { i := smallRequest("COMPLEX", classOn()); i.Request.Pinned = true; return i }(),
		"class routing off":    smallRequest("COMPLEX", testPolicy()),
		"profile chose small":  func() Input { i := smallRequest("COMPLEX", classOn()); i.Request.Profile = "cheap"; return i }(),
	} {
		if d, _ := Decide(in); d.Advice != "" {
			t.Errorf("%s: advice %q, want none", name, d.Advice)
		}
	}
}

// C: with routing.raise_complex the same task moves one tier up, to mid, and
// never past it.
func TestDecideRaisesComplexOnSmallWhenAsked(t *testing.T) {
	pol := classOn()
	pol.RaiseComplex = true
	d, n := Decide(smallRequest("COMPLEX", pol))
	if d.Model.ID != "mid-model" || n.Verdict != contract.VerdictClassUp || n.TierTo != contract.TierMid || d.Verdict != core.VerdictUpshift || d.Advice != core.AdviceComplexOnSmall {
		t.Fatalf("model %q verdict %q tier %q core %q advice %q, want mid-model class_up mid UPSHIFT and the advice as fallback",
			d.Model.ID, n.Verdict, n.TierTo, d.Verdict, d.Advice)
	}
	pinned := smallRequest("COMPLEX", pol)
	pinned.Request.Pinned = true
	if d, _ := Decide(pinned); d.Model.ID != "small-model" {
		t.Fatalf("pinned: model %q, want small-model", d.Model.ID)
	}
	hinted := smallRequest("COMPLEX", pol)
	hinted.Request.Hint.Tier = contract.TierSmall
	if d, _ := Decide(hinted); d.Model.ID != "small-model" || d.Advice != core.AdviceComplexOnSmall {
		t.Fatalf("hint small: model %q advice %q, want small-model and the advice", d.Model.ID, d.Advice)
	}
	ruled := smallRequest("COMPLEX", pol)
	ruled.Request.Profile = "cheap"
	if d, _ := Decide(ruled); d.Model.ID != "small-model" || d.Advice != "" {
		t.Fatalf("profile small: model %q advice %q, want small-model and no advice", d.Model.ID, d.Advice)
	}
	mid := classInput("COMPLEX", true, nil, pol)
	mid.Request.RequestedModel = "mid-model"
	if d, n := Decide(mid); d.Model.ID != "mid-model" || n.Verdict != contract.VerdictKept {
		t.Fatalf("mid request: model %q verdict %q, want mid-model kept", d.Model.ID, n.Verdict)
	}
}
