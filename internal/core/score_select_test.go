// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core_test

import (
	"reflect"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/core"
)

// fakeScores installs a ScoreHook backed by a map and restores it afterwards.
func fakeScores(t *testing.T, scores map[string]float64) {
	t.Helper()
	old := core.ScoreHook
	t.Cleanup(func() { core.ScoreHook = old })
	core.ScoreHook = func(_, id string) (float64, bool) {
		s, ok := scores[id]
		return s, ok
	}
}

// priceResolver is a minimal Resolver with per-id prices and explicit_only.
type priceResolver struct {
	price    map[string]float64
	explicit map[string]bool
	tier     core.Tier // catalog tier of every id; zero value means mid
}

func (r priceResolver) ModelFor(h string, t core.Tier) core.Model {
	return core.Model{Harness: h, Tier: t}
}
func (r priceResolver) LookupByID(h, id string) (core.Model, bool) {
	p, ok := r.price[id]
	if !ok {
		return core.Model{}, false
	}
	tier := core.TierMid
	if r.tier != 0 {
		tier = r.tier
	}
	return core.Model{ID: id, Harness: h, Tier: tier, InputM: p, OutputM: p, Native: id}, true
}
func (r priceResolver) SavingsRatio(_, _ core.Model) float64        { return 0 }
func (r priceResolver) EffortFor(_, _ string, e core.Effort) string { return e.String() }
func (r priceResolver) IsExplicitOnly(_, id string) bool            { return r.explicit[id] }
func (r priceResolver) IsExactID(_, id string) bool                 { _, ok := r.price[id]; return ok }

var scoreHarnesses = []struct {
	harness string
	caps    core.HarnessCapabilities
}{
	{"claude-code", core.ClaudeCodeCaps},
	{"codex", core.CodexCaps},
	{"cursor", core.CursorCaps},
	{"antigravity", core.AntigravityCaps},
}

var (
	scoreIDs    = []string{"m-10", "m-40", "m-70", "m-90"}
	scoreValues = map[string]float64{"m-10": 10, "m-40": 40, "m-70": 70, "m-90": 90}
)

// need(small)=10, need(mid)=10+80*0.6=58, need(frontier)=10+80*0.95=86.
func TestScoreWould_NeedScalesWithTierAcrossHarnesses(t *testing.T) {
	fakeScores(t, scoreValues)
	for _, h := range scoreHarnesses {
		for _, tc := range []struct {
			name, prompt string
			want         string
		}{
			{"small", "rename the variable", "m-10"},
			{"mid", "implement the CSV export feature", "m-70"},
		} {
			t.Run(h.harness+"/"+tc.name, func(t *testing.T) {
				d := core.Route(tc.prompt, h.harness, "", nil)
				d.PlanSession(h.caps, nil, core.KnownSession(scoreIDs), "")
				if d.ScoreWould != tc.want {
					t.Fatalf("ScoreWould=%q want %q (tier %s)", d.ScoreWould, tc.want, d.Tier)
				}
			})
		}
		t.Run(h.harness+"/frontier", func(t *testing.T) {
			// A frontier tier without risk or COMPLEX only arises from a lift,
			// which abstains; hand-build one to pin f(frontier).
			d := core.Decision{Harness: h.harness, Tier: core.TierFrontier, Complexity: core.Medium, Confident: true}
			d.RecordScoreWould(nil, core.KnownSession(scoreIDs))
			if d.ScoreWould != "m-90" {
				t.Fatalf("ScoreWould=%q want m-90", d.ScoreWould)
			}
		})
	}
}

func TestScoreWould_AbstainsWithoutCompleteScores(t *testing.T) {
	session := core.KnownSession(scoreIDs)
	mid := func() core.Decision {
		return core.Decision{Harness: "claude-code", Tier: core.TierMid, Complexity: core.Medium, Confident: true}
	}
	t.Run("no hook", func(t *testing.T) {
		old := core.ScoreHook
		defer func() { core.ScoreHook = old }()
		core.ScoreHook = nil
		d := mid()
		d.RecordScoreWould(nil, session)
		if d.ScoreWould != "" {
			t.Fatalf("ScoreWould=%q, want abstain", d.ScoreWould)
		}
	})
	t.Run("one candidate has no score", func(t *testing.T) {
		fakeScores(t, map[string]float64{"m-10": 10, "m-40": 40, "m-90": 90})
		d := mid()
		d.RecordScoreWould(nil, session)
		if d.ScoreWould != "" {
			t.Fatalf("ScoreWould=%q, want abstain", d.ScoreWould)
		}
	})
	t.Run("a single candidate has no range", func(t *testing.T) {
		fakeScores(t, scoreValues)
		d := mid()
		d.RecordScoreWould(nil, core.KnownSession([]string{"m-70"}))
		if d.ScoreWould != "" {
			t.Fatalf("ScoreWould=%q, want abstain", d.ScoreWould)
		}
	})
	t.Run("unknown session", func(t *testing.T) {
		fakeScores(t, scoreValues)
		d := mid()
		d.RecordScoreWould(nil, core.UnknownSession())
		if d.ScoreWould != "" {
			t.Fatalf("ScoreWould=%q, want abstain", d.ScoreWould)
		}
	})
	t.Run("an exhausted id without a score does not block", func(t *testing.T) {
		fakeScores(t, map[string]float64{"m-10": 10, "m-40": 40, "m-70": 70})
		s := session
		s.Exhausted = []string{"m-90"}
		d := mid()
		d.RecordScoreWould(nil, s)
		// range 10..70, need 10+60*0.6=46 -> m-70
		if d.ScoreWould != "m-70" {
			t.Fatalf("ScoreWould=%q, want m-70", d.ScoreWould)
		}
	})
}

// The same models, a narrower list: the need is relative to the session.
func TestScoreWould_RelativeToTheSessionRange(t *testing.T) {
	fakeScores(t, scoreValues)
	pick := func(ids ...string) string {
		d := core.Decision{Harness: "codex", Tier: core.TierMid, Complexity: core.Medium, Confident: true}
		d.RecordScoreWould(nil, core.KnownSession(ids))
		return d.ScoreWould
	}
	if got := pick("m-10", "m-40", "m-70", "m-90"); got != "m-70" {
		t.Fatalf("wide list: %q, want m-70", got)
	}
	// 70..90, need 70+20*0.6=82: m-70 no longer reaches mid.
	if got := pick("m-70", "m-90"); got != "m-90" {
		t.Fatalf("narrow list: %q, want m-90", got)
	}
	// Small always takes the cheapest-by-order id that reaches the floor (lo).
	d := core.Decision{Harness: "codex", Tier: core.TierSmall, Complexity: core.Simple, Confident: true}
	d.RecordScoreWould(nil, core.KnownSession([]string{"m-70", "m-90"}))
	if d.ScoreWould != "m-70" {
		t.Fatalf("small on narrow list: %q, want m-70", d.ScoreWould)
	}
}

func TestScoreWould_RiskComplexAndLiftsStayOnTheCatalogPath(t *testing.T) {
	fakeScores(t, scoreValues)
	session := core.KnownSession(scoreIDs)
	for _, tc := range []struct {
		name string
		d    core.Decision
	}{
		{"complex", core.Decision{Complexity: core.Complex, Tier: core.TierFrontier, Confident: true}},
		{"risk category", core.Decision{Complexity: core.Medium, Tier: core.TierMid, Confident: true, Risk: []string{"auth"}}},
		{"risk floor", core.Decision{Complexity: core.Medium, Tier: core.TierMid, Confident: true, RiskFloor: true}},
		{"operator floor", core.Decision{Complexity: core.Medium, Tier: core.TierMid, Confident: true, PolicyFloor: true}},
		{"graph escalation", core.Decision{Complexity: core.Medium, Tier: core.TierMid, Confident: true, GraphEscalated: true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := tc.d
			d.Harness = "claude-code"
			d.RecordScoreWould(nil, session)
			if d.ScoreWould != "" {
				t.Fatalf("ScoreWould=%q, want abstain", d.ScoreWould)
			}
		})
	}
	// Through Route: a real risk prompt and a real COMPLEX prompt.
	for _, p := range []string{
		"rotate the production api key and update the secrets",
		"rearchitect the payment system across services",
	} {
		d := core.Route(p, "claude-code", "", nil)
		d.PlanSession(core.ClaudeCodeCaps, nil, session, "")
		if d.ScoreWould != "" {
			t.Fatalf("%q: ScoreWould=%q (complexity %s, risk %v), want abstain", p, d.ScoreWould, d.Complexity, d.Risk)
		}
	}
}

// Shadow by default: with the hook linked but the flag off, the plan is
// byte-identical to the plan with no hook at all.
func TestScoreShadow_DoesNotChangeThePlan(t *testing.T) {
	t.Setenv(core.ScoreRoutingEnv, "")
	prompts := append(finalInvariantPrompts(), "rename the variable", "implement the CSV export feature")
	for _, h := range scoreHarnesses {
		session := core.KnownSession(scoreIDs)
		for _, p := range prompts {
			for _, cur := range []string{"", "m-90", "m-10"} {
				plan := func(withHook bool) core.RewritePlan {
					old := core.ScoreHook
					defer func() { core.ScoreHook = old }()
					core.ScoreHook = nil
					if withHook {
						fakeScores(t, scoreValues)
					}
					d := core.Route(p, h.harness, cur, nil)
					d.RequestedID = cur
					return d.PlanSession(h.caps, nil, session, cur)
				}
				if a, b := plan(false), plan(true); !reflect.DeepEqual(a, b) {
					t.Fatalf("%s %.30q cur=%q: shadow changed the plan\nwithout: %+v\nwith:    %+v", h.harness, p, cur, a, b)
				}
			}
		}
	}
}

// liveDecision is a confident, checked downshift from the strongest model.
func liveDecision(harness string, tier core.Tier) core.Decision {
	return core.Decision{
		Harness: harness, Tier: tier, Complexity: core.Medium, Confident: true,
		Verdict: core.VerdictDownshift, SafeVerdict: core.VerdictDownshift, Checked: true,
		RequestedID:  "m-90",
		CurrentModel: core.Model{ID: "m-90", Harness: harness},
		Model:        core.Model{ID: "not-in-session", Harness: harness},
	}
}

func TestScoreLive_PicksTheCheapestPassingID(t *testing.T) {
	// m-40 reaches mid on the list below but costs more than m-70.
	scores := map[string]float64{"m-10": 10, "m-40": 60, "m-70": 70, "m-90": 90}
	fakeScores(t, scores)
	res := priceResolver{price: map[string]float64{"m-10": 1, "m-40": 9, "m-70": 3, "m-90": 20}}
	t.Setenv(core.ScoreRoutingEnv, "1")
	for _, h := range scoreHarnesses {
		t.Run(h.harness, func(t *testing.T) {
			// need(mid)=10+80*0.6=58: m-40 (60), m-70, m-90 pass; m-70 is cheapest.
			d := liveDecision(h.harness, core.TierMid)
			plan := d.PlanSession(h.caps, res, core.KnownSession(scoreIDs), "m-90")
			if plan.Model.ID != "m-70" || !plan.RewriteModel {
				t.Fatalf("plan=%s rewrite=%v, want m-70", plan.Model.ID, plan.RewriteModel)
			}
			if d.ScoreWould != "m-70" || !d.ScoreApplied {
				t.Fatalf("ScoreWould=%q applied=%v", d.ScoreWould, d.ScoreApplied)
			}
			// The comparison is against the catalog-tier path, which with every id
			// at mid tier takes the first id (m-10, price 1): the pick costs 3x.
			if !d.ScoreDiffers || d.ScoreWouldCostRatio < 3-1e-9 || d.ScoreWouldCostRatio > 3+1e-9 {
				t.Fatalf("differs=%v ratio=%v, want true 3", d.ScoreDiffers, d.ScoreWouldCostRatio)
			}
		})
	}
	// Flag off: same decision records the pick but does not apply it.
	t.Setenv(core.ScoreRoutingEnv, "")
	d := liveDecision("claude-code", core.TierMid)
	d.PlanSession(core.ClaudeCodeCaps, res, core.KnownSession(scoreIDs), "m-90")
	if d.ScoreWould != "m-70" || d.ScoreApplied {
		t.Fatalf("shadow: ScoreWould=%q applied=%v", d.ScoreWould, d.ScoreApplied)
	}
}

func TestScoreLive_KeepsQuotaExplicitOnlyAndHolds(t *testing.T) {
	fakeScores(t, scoreValues)
	t.Setenv(core.ScoreRoutingEnv, "1")
	t.Run("exhausted id is not picked", func(t *testing.T) {
		s := core.KnownSession(scoreIDs)
		s.Exhausted = []string{"m-70"}
		d := liveDecision("claude-code", core.TierMid)
		d.PlanSession(core.ClaudeCodeCaps, nil, s, "m-90")
		// range 10..90 over m-10,m-40,m-90 stays 10..90, need 58 -> m-90
		if d.ScoreWould != "m-90" {
			t.Fatalf("ScoreWould=%q, want m-90", d.ScoreWould)
		}
	})
	t.Run("explicit_only id is not picked or counted", func(t *testing.T) {
		res := priceResolver{
			price:    map[string]float64{"m-10": 1, "m-40": 2, "m-70": 3, "m-90": 4},
			explicit: map[string]bool{"m-90": true},
			tier:     core.TierFrontier,
		}
		d := liveDecision("codex", core.TierFrontier)
		d.PlanSession(core.CodexCaps, res, core.KnownSession(scoreIDs), "m-70")
		// range over m-10..m-70 = 10..70, need 10+60*0.95=67 -> m-70
		if d.ScoreWould != "m-70" {
			t.Fatalf("ScoreWould=%q, want m-70", d.ScoreWould)
		}
	})
	t.Run("guardrail hold stops the live pick", func(t *testing.T) {
		d := liveDecision("claude-code", core.TierMid)
		d.SafeVerdict = core.VerdictOK
		plan := d.PlanSession(core.ClaudeCodeCaps, nil, core.KnownSession(scoreIDs), "m-90")
		if plan.RewriteModel || d.ScoreApplied {
			t.Fatalf("held decision was rewritten to %q", plan.Model.ID)
		}
	})
	t.Run("unconfident decision never goes above the current model", func(t *testing.T) {
		d := liveDecision("claude-code", core.TierMid)
		d.Confident = false
		d.RequestedID, d.CurrentModel.ID = "m-40", "m-40"
		d.Verdict, d.SafeVerdict = core.VerdictUnknown, core.VerdictUnknown
		d.PlanSession(core.ClaudeCodeCaps, nil, core.KnownSession(scoreIDs), "m-40")
		if d.ScoreWould != "" || d.ScoreApplied {
			t.Fatalf("ScoreWould=%q applied=%v, want abstain above the current model", d.ScoreWould, d.ScoreApplied)
		}
	})
}

// Route + PlanSession keep the final-decision invariant with the flag on.
func TestFinalDecision_GuardrailsHoldWithScoreRoutingOn(t *testing.T) {
	t.Setenv(core.ScoreRoutingEnv, "1")
	fakeScores(t, scoreValues)
	n := 0
	for _, p := range finalInvariantPrompts() {
		for _, cur := range []string{"", "m-90", "m-10"} {
			for _, h := range scoreHarnesses {
				d := core.Route(p, h.harness, cur, nil)
				d.RequestedID = cur
				d.PlanSession(h.caps, nil, core.KnownSession(scoreIDs), cur)
				n++
				if err := finalDecisionError(d); err != nil {
					t.Errorf("%s prompt=%.40q current=%q: %v", h.harness, p, cur, err)
				}
			}
		}
	}
	for _, p := range finalInvariantPrompts() {
		for _, cur := range finalInvariantCurrents {
			d := core.Route(p, "claude-code", cur, cat)
			d.PlanSession(core.ClaudeCodeCaps, cat, core.KnownSession([]string{"claude-haiku-5-5", "claude-sonnet-5-5", "claude-opus-5-5"}), cur)
			if err := finalDecisionError(d); err != nil {
				t.Errorf("catalog prompt=%.40q current=%q: %v", p, cur, err)
			}
		}
	}
	t.Logf("%d decisions checked", n)
}
