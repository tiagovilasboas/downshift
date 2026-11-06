// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package discovery

import (
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/tiagovilasboas/downshift/internal/catalog"
	"github.com/tiagovilasboas/downshift/internal/core"
)

// Catalog is what discovery needs from the catalog: metadata for an id (Order)
// and the session stand-in (MergeReported's union). *catalog.Catalog satisfies
// it. SessionCandidates, LookupByID and IsExplicitOnly serve the same stand-in
// the model-availability surfaces show; the catalog ranks ids, it never adds
// one beyond that stand-in.
type Catalog interface {
	EntryFor(harness, modelID string) (catalog.Entry, bool)
	IsExactID(harness, id string) bool
	SessionCandidates(harness string) []string
	LookupByID(harness, modelID string) (core.Model, bool)
	IsExplicitOnly(harness, modelID string) bool
}

// Ranked is the routing view of a discovery result.
type Ranked struct {
	Metadata []core.SessionModel `json:"model_metadata,omitempty"`
	// IDs are the usable models, least to most capable, one per family
	// (the newest). This is the list a hook may write from.
	IDs []string `json:"ordered"`
	// Unranked were reported by the harness but the catalog has no tier for
	// them (no exact, alias or family match). They are never routing targets
	// until a tier is assigned.
	Unranked []string `json:"unranked,omitempty"`
	// Superseded were reported but a newer model of the same family exists.
	Superseded []string `json:"superseded,omitempty"`
	// FamilyOnly were matched to the catalog only through their family: the
	// catalog does not list the id (or an alias) itself, so the tier is
	// inherited. The harness may offer a newer or an older version.
	FamilyOnly []FamilyMatch `json:"family_only,omitempty"`
}

// FamilyMatch records a discovered id that inherited its tier from a catalog family.
type FamilyMatch struct {
	ID        string `json:"id"`
	CatalogID string `json:"catalog_id"`
}

var tierRank = map[string]int{"small": 0, "mid": 1, "frontier": 2}

type candidate struct {
	m     Model
	e     catalog.Entry
	group string
}

// Order ranks discovered models with catalog metadata. It never parses a
// model name to decide capability: tier comes from the catalog, and "newest"
// comes from the provider's created_at, falling back to comparing the numbers
// inside the ids only to break a tie within one family.
func Order(harness string, models []Model, cat Catalog) Ranked {
	var r Ranked
	groups := map[string][]candidate{}
	var groupOrder []string
	for _, m := range models {
		r.Metadata = append(r.Metadata, core.SessionModel{ID: m.ID, Tier: m.Tier, Efforts: m.Efforts, Harness: harness})
		e, ok := rankEntry(harness, m, cat)
		if !ok {
			r.Unranked = append(r.Unranked, m.ID)
			continue
		}
		if _, ranked := tierRank[strings.ToLower(e.Tier)]; !ranked {
			r.Unranked = append(r.Unranked, m.ID)
			continue
		}
		if m.Tier == "" && !m.Gateway && cat != nil && !cat.IsExactID(harness, m.ID) {
			r.FamilyOnly = append(r.FamilyOnly, FamilyMatch{ID: m.ID, CatalogID: e.ID})
		}
		key := strings.ToLower(e.Family) + "|" + strings.ToLower(e.Effort)
		if e.Family == "" {
			key = "id:" + e.ID
		}
		if _, seen := groups[key]; !seen {
			groupOrder = append(groupOrder, key)
		}
		groups[key] = append(groups[key], candidate{m: m, e: e, group: key})
	}

	var winners []candidate
	for _, key := range groupOrder {
		cs := groups[key]
		best := cs[0]
		for _, c := range cs[1:] {
			if newer(c.m, best.m) {
				best = c
			}
		}
		winners = append(winners, best)
		for _, c := range cs {
			if c.m.ID != best.m.ID {
				r.Superseded = append(r.Superseded, c.m.ID)
			}
		}
	}

	sort.SliceStable(winners, func(i, j int) bool {
		a, b := winners[i], winners[j]
		if ra, rb := tierRank[strings.ToLower(a.e.Tier)], tierRank[strings.ToLower(b.e.Tier)]; ra != rb {
			return ra < rb
		}
		// Within a tier the order is the model id (owner decision 2026-10-31:
		// the product uses no price, so a cheaper-first tiebreak is gone).
		return a.m.ID < b.m.ID
	})
	for _, w := range winners {
		r.IDs = append(r.IDs, w.m.ID)
	}
	return r
}

// rankEntry returns the catalog row that gives a discovered model its tier.
// A gateway id is ranked only by an allowlist row that names it exactly and
// routes live; the vendor prefix is never stripped for it, so a native
// homonym's tier cannot be reused.
func rankEntry(harness string, m Model, cat Catalog) (catalog.Entry, bool) {
	if m.Gateway {
		return gatewayRankEntry(harness, m.ID, cat)
	}
	var e catalog.Entry
	var ok bool
	if cat != nil {
		e, ok = cat.EntryFor(harness, m.ID)
	}
	nativeTier := strings.ToLower(m.Tier)
	if _, valid := tierRank[nativeTier]; valid {
		// A provider descriptor outranks the fallback catalog. Keep native
		// IDs separate instead of deduplicating them using catalog families.
		e = catalog.Entry{ID: m.ID, Harness: harness, Tier: nativeTier}
		ok = true
	} else if m.Tier != "" {
		// Explicit malformed capability evidence cannot silently fall back.
		ok = false
	}
	return e, ok
}

// gatewayRankEntry resolves a discovered gateway id only against the gateway
// rows of the allowlist, by exact slug. A shadow or explicit-only row does not
// rank it: a row must route live.
func gatewayRankEntry(harness, id string, cat Catalog) (catalog.Entry, bool) {
	if cat == nil {
		return catalog.Entry{}, false
	}
	routeID := catalog.AllowlistProvider + "/" + id
	if !cat.IsExactID(harness, routeID) {
		return catalog.Entry{}, false
	}
	e, ok := cat.EntryFor(harness, routeID)
	if !ok || e.Routing == "explicit_only" || e.Routing == catalog.RoutingShadow {
		return catalog.Entry{}, false
	}
	return e, true
}

// newer reports whether a is a later release than b.
func newer(a, b Model) bool {
	if !a.CreatedAt.IsZero() && !b.CreatedAt.IsZero() && !a.CreatedAt.Equal(b.CreatedAt) {
		return a.CreatedAt.After(b.CreatedAt)
	}
	return compareNumbers(a.ID, b.ID) > 0
}

var digits = regexp.MustCompile(`\d+`)

// compareNumbers compares the numeric runs of two ids left to right
// ("claude-sonnet-4.6" > "claude-sonnet-4.5"), then falls back to string order.
func compareNumbers(a, b string) int {
	na, nb := digits.FindAllString(a, -1), digits.FindAllString(b, -1)
	for i := 0; i < len(na) && i < len(nb); i++ {
		x, _ := strconv.Atoi(na[i])
		y, _ := strconv.Atoi(nb[i])
		if x != y {
			if x > y {
				return 1
			}
			return -1
		}
	}
	if len(na) != len(nb) {
		if len(na) > len(nb) {
			return 1
		}
		return -1
	}
	return strings.Compare(a, b)
}
