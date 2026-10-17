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
