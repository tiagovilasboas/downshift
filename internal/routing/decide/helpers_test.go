// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package decide

import (
	"time"

	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/routing/contract"
	"github.com/tiagovilasboas/downshift/internal/routing/policy"
)

const h contract.HarnessID = "test-harness"

var now = time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)

// fakeCatalog: ids in models are known to LookupByID; ids in exact are exact
// automatic targets; byTier is what ModelFor answers.
type fakeCatalog struct {
	models   map[string]core.Model
	exact    map[string]bool
	explicit map[string]bool
	byTier   map[core.Tier]core.Model
	boom     bool
}

func (f fakeCatalog) IsExactID(_, id string) bool      { return f.exact[id] }
func (f fakeCatalog) IsExplicitOnly(_, id string) bool { return f.explicit[id] }
func (f fakeCatalog) LookupByID(_, id string) (core.Model, bool) {
	if f.boom {
		panic("catalog exploded")
	}
	m, ok := f.models[id]
	return m, ok
}
func (f fakeCatalog) ModelFor(_ string, t core.Tier) core.Model { return f.byTier[t] }

func model(id string, t core.Tier) core.Model {
	return core.Model{ID: id, Tier: t, Harness: string(h)}
}

func newCatalog() fakeCatalog {
	s, m, f := model("small-model", core.TierSmall), model("mid-model", core.TierMid), model("frontier-model", core.TierFrontier)
	return fakeCatalog{
		models: map[string]core.Model{"small-model": s, "mid-model": m, "frontier-model": f,
			"explicit-model": model("explicit-model", core.TierSmall)},
		exact:    map[string]bool{"small-model": true, "mid-model": true, "frontier-model": true},
		explicit: map[string]bool{"explicit-model": true},
		byTier:   map[core.Tier]core.Model{core.TierSmall: s, core.TierMid: m, core.TierFrontier: f},
	}
}

func testPolicy() *policy.Policy {
	return &policy.Policy{
		CriticalThreshold: 80,
		Profiles: []policy.Profile{
			{Name: "cheap", Tier: contract.TierSmall, Harness: h},
			{Name: "reviewer", Tier: contract.TierMid, Harness: h},
			{Name: "top", Tier: contract.TierFrontier, Harness: h},
			{Name: "elsewhere", Tier: contract.TierSmall, Harness: "other"},
		},
		Rules: []policy.Rule{
			{ID: "gated-docs", Enabled: true, Kinds: []contract.Kind{contract.KindDocs}, Tier: contract.TierSmall,
				Gates: []policy.Gate{{Harness: h, Window: contract.WindowShort, Below: 50}}},
			{ID: "docs", Enabled: true, Kinds: []contract.Kind{contract.KindDocs}, Tier: contract.TierMid},
			{ID: "scrap", Enabled: true, Kinds: []contract.Kind{contract.KindScrap}, Profile: "cheap"},
			{ID: "ghost", Enabled: true, Kinds: []contract.Kind{contract.KindCommand}, Profile: "missing"},
		},
	}
}

func req(mod func(*contract.Request)) contract.Request {
	r := contract.Request{Harness: h, RequestedModel: "frontier-model"}
	if mod != nil {
		mod(&r)
	}
	return r
}
