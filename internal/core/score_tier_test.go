package core_test

import (
	"testing"

	"github.com/tiagovilasboas/downshift/internal/core"
)

// tieredResolver gives each id a catalog tier and a price.
type tieredResolver struct {
	tier  map[string]core.Tier
	price map[string]float64
}

func (r tieredResolver) ModelFor(h string, t core.Tier) core.Model {
	for id, tt := range r.tier {
		if tt == t {
			return core.Model{ID: id, Harness: h, Tier: t, InputM: r.price[id], OutputM: r.price[id], Native: id}
		}
	}
	return core.Model{Harness: h, Tier: t}
}
func (r tieredResolver) LookupByID(h, id string) (core.Model, bool) {
	t, ok := r.tier[id]
	if !ok {
		return core.Model{}, false
	}
	return core.Model{ID: id, Harness: h, Tier: t, InputM: r.price[id], OutputM: r.price[id], Native: id}, true
}
func (r tieredResolver) SavingsRatio(_, _ core.Model) float64        { return 0.5 }
func (r tieredResolver) EffortFor(_, _ string, e core.Effort) string { return e.String() }
func (r tieredResolver) IsExplicitOnly(_, _ string) bool             { return false }
func (r tieredResolver) IsExactID(_, id string) bool                 { _, ok := r.tier[id]; return ok }

// AUDIT claim 4: live score routing must never pick a model whose catalog tier
// is below the decision's guardrail-checked tier.
func TestScoreTier_ScoreRoutingNeverBelowDecisionTier(t *testing.T) {
	t.Setenv(core.ScoreRoutingEnv, "1")
	fakeScores(t, map[string]float64{"s-lo": 50, "s-hi": 95, "m-1": 96, "f-1": 100})
	res := tieredResolver{
		tier:  map[string]core.Tier{"s-lo": core.TierSmall, "s-hi": core.TierSmall, "m-1": core.TierMid, "f-1": core.TierFrontier},
		price: map[string]float64{"s-lo": 1, "s-hi": 2, "m-1": 5, "f-1": 20},
	}
	session := core.KnownSession([]string{"s-lo", "s-hi", "m-1", "f-1"})

	d := core.Route("implement the CSV export feature", "claude-code", "f-1", res)
	d.RequestedID = "f-1"
	t.Logf("route: complexity=%s tier=%s confident=%v verdict=%s safe=%s risk=%v corrections=%v",
		d.Complexity, d.Tier, d.Confident, d.Verdict, d.SafeVerdict, d.Risk, d.Corrections)
	if d.Tier != core.TierMid || d.SafeVerdict != core.VerdictDownshift {
		t.Skipf("precondition not met: need a clean mid-tier downshift, got tier=%s safe=%s", d.Tier, d.SafeVerdict)
	}
	plan := d.PlanSession(core.ClaudeCodeCaps, res, session, "f-1")
	m, _ := res.LookupByID("claude-code", plan.Model.ID)
	if plan.RewriteModel && m.Tier < d.Tier {
		t.Fatalf("score routing wrote %q (catalog tier %s) for a %s-tier decision; ScoreApplied=%v",
			plan.Model.ID, m.Tier, d.Tier, d.ScoreApplied)
	}
}
