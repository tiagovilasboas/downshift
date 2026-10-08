// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package discovery

import (
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/tiagovilasboas/downshift/internal/catalog"
)

// Catalog is what Order needs from the catalog: metadata for an id.
// *catalog.Catalog satisfies it.
type Catalog interface {
	EntryFor(harness, modelID string) (catalog.Entry, bool)
	IsExactID(harness, id string) bool
}

// Ranked is the routing view of a discovery result.
type Ranked struct {
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
// model name to decide capability: tier and price come from the catalog, and
// "newest" comes from the provider's created_at, falling back to comparing the
// numbers inside the ids only to break a tie within one family.
func Order(harness string, models []Model, cat Catalog) Ranked {
	var r Ranked
	groups := map[string][]candidate{}
	var groupOrder []string
	for _, m := range models {
		e, ok := cat.EntryFor(harness, m.ID)
		if !ok {
			r.Unranked = append(r.Unranked, m.ID)
			continue
		}
		if _, ranked := tierRank[strings.ToLower(e.Tier)]; !ranked {
			r.Unranked = append(r.Unranked, m.ID)
			continue
		}
		if !cat.IsExactID(harness, m.ID) {
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
		if a.e.OutputCostM != b.e.OutputCostM {
			return a.e.OutputCostM < b.e.OutputCostM
		}
		return a.m.ID < b.m.ID
	})
	for _, w := range winners {
		r.IDs = append(r.IDs, w.m.ID)
	}
	return r
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
