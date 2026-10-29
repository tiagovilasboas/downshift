package core_test

import (
	"testing"

	"github.com/tiagovilasboas/downshift/internal/core"
)

// AUDIT: an UPSHIFT verdict must never write a model whose catalog tier is
// below the current model's (or below the decision tier).
func TestUpshiftTier_UpshiftNeverWritesLowerCatalogTier(t *testing.T) {
	res := tieredResolver{
		tier:  map[string]core.Tier{"s-a": core.TierSmall, "m-b": core.TierMid, "s-c": core.TierSmall},
		price: map[string]float64{"s-a": 1, "m-b": 5, "s-c": 1},
	}
	// Session order is not least-to-most capable (a hook/discovered list in
	// arbitrary order); every id is known to the catalog.
	session := core.KnownSession([]string{"s-a", "m-b", "s-c"})
	p := "rearchitect the payment flow across services and migrate the auth tokens"
	d := core.Route(p, "claude-code", "m-b", res)
	d.Baseline.RequestedID = "m-b"
	plan := d.PlanSession(core.ClaudeCodeCaps, res, session, "m-b")
	t.Logf("cx=%s tier=%s conf=%v risk=%v verdict=%s safe=%s corrections=%v plan=%s rewrite=%v",
		d.Complexity, d.Tier, d.Confident, d.Risk, d.Verdict, d.SafeVerdict, d.Corrections, plan.Model.ID, plan.RewriteModel)
	if !plan.RewriteModel {
		return
	}
	m, _ := res.LookupByID("claude-code", plan.Model.ID)
	if m.Tier < core.TierMid {
		t.Fatalf("UPSHIFT verdict (tier %s, risk %v) wrote %q (catalog tier %s) over current m-b (mid)",
			d.Tier, d.Risk, plan.Model.ID, m.Tier)
	}
}

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
