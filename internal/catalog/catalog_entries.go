// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package catalog

import (
	"slices"
	"strings"

	"github.com/tiagovilasboas/downshift/internal/core"
)

// --- Accessors for commands (models list / check / pull) ---

// Entries returns all raw catalog entries (including unknown-tier ones).
func (c *Catalog) Entries() []Entry {
	return c.entries
}

// EffortValue translates a core.Effort to the harness-native string for a
// given model entry. Falls back to effort.String() if no map entry exists.
func EffortValue(e Entry, effort core.Effort) string {
	if e.EffortMap != nil {
		if v, ok := e.EffortMap[effort.String()]; ok {
			return v
		}
	}
	return effort.String()
}

// EntryFor returns the raw Entry for a given harness+modelID. Uses the same
// three-strategy lookup as LookupByID (exact → alias → family prefix), including
// OpenRouter "provider/model-id" normalisation.
func (c *Catalog) EntryFor(harness, modelID string) (Entry, bool) {
	if modelID == "" {
		return Entry{}, false
	}
	if slug, ok := gatewaySlug(modelID); ok {
		return c.gatewayEntryFor(harness, slug)
	}
	normalised := normaliseModelID(modelID)

	// Exact and alias match — try both original and normalised.
	for _, id := range uniqueStrings(modelID, normalised) {
		for _, e := range c.entries {
			if e.Harness != harness {
				continue
			}
			if e.ID == id {
				return e, true
			}
			for _, a := range e.Aliases {
				if a == id {
					return e, true
				}
			}
			if e.NativeName != "" && e.NativeName == id {
				return e, true
			}
		}
	}

	// Family prefix fallback, full slug first (see LookupByID).
	for _, lower := range familyCandidates(modelID) {
		for _, e := range c.entries {
			if e.Harness == harness && e.Family != "" {
				if strings.HasPrefix(lower, strings.ToLower(e.Family)) {
					return e, true
				}
			}
		}
	}
	return Entry{}, false
}

// gatewayEntryFor is EntryFor for the "openrouter/<slug>" form: an OpenRouter
// row by exact slug or alias, then by family prefix. No other row matches.
func (c *Catalog) gatewayEntryFor(harness, slug string) (Entry, bool) {
	if slug == "" {
		return Entry{}, false
	}
	for _, e := range c.entries {
		if isGatewayProvider(e.Provider) && e.Harness == harness && isGatewayName(e, slug) {
			return e, true
		}
	}
	lower := strings.ToLower(slug)
	for _, e := range c.entries {
		if isGatewayProvider(e.Provider) && e.Harness == harness && e.Family != "" &&
			strings.HasPrefix(lower, strings.ToLower(e.Family)) {
			return e, true
		}
	}
	return Entry{}, false
}

// isGatewayName reports whether slug is the id or an alias of a gateway row.
func isGatewayName(e Entry, slug string) bool {
	return e.ID == slug || slices.Contains(e.Aliases, slug)
}

// IsExplicitOnly implements core.Resolver. Returns true when the model ID is
// marked routing:"explicit_only" — meaning it is a valid current selection
// but must never be chosen as an automatic routing target.
func (c *Catalog) IsExplicitOnly(harness, modelID string) bool {
	if modelID == "" {
		return false
	}
	entry, ok := c.EntryFor(harness, modelID)
	if !ok {
		return false
	}
	return entry.Routing == "explicit_only" || entry.Routing == RoutingShadow
}

// automaticRoutable reports whether a row may be indexed as an automatic
// target. Explicit-only and shadow rows are never indexed.
func automaticRoutable(e Entry) bool {
	return e.Routing != "explicit_only" && e.Routing != RoutingShadow && e.Routing != RoutingLive
}

// IsShadow reports whether modelID resolves to an allowlist shadow row. Core
// uses it to keep shadow rows out of every session selection path.
func (c *Catalog) IsShadow(harness, modelID string) bool {
	entry, ok := c.EntryFor(harness, modelID)
	return ok && entry.Routing == RoutingShadow
}

// ShadowGateways lists the allowlisted gateway rows (Routing "shadow") as
// models. Harness-agnostic: the allowlist is a user fact, not a harness fact.
// Rows carry only the tier and id; no price is invented for them.
func (c *Catalog) ShadowGateways() []core.Model {
	return c.gatewayRows(RoutingShadow)
}

// LiveGateways lists the allowlisted gateway rows the user made live (Routing
// "live"). core targets them only for the harness and tier they name.
func (c *Catalog) LiveGateways() []core.Model {
	return c.gatewayRows(RoutingLive)
}

// gatewayRows lists the gateway rows with the given routing as models.
func (c *Catalog) gatewayRows(routing string) []core.Model {
	var out []core.Model
	for _, e := range c.entries {
		if !isGatewayProvider(e.Provider) || e.Routing != routing || e.ID == "" {
			continue
		}
		tier, ok := parseTier(e.Tier)
		if !ok {
			continue
		}
		out = append(out, core.Model{ID: e.ID, Tier: tier, Harness: e.Harness, Gateway: e.Provider})
	}
	return out
}

// EffortFor implements core.Resolver. It translates a core.Effort level to the
// harness-native string for the given model ID using the entry's effort_map.
// Falls back to effort.String() ("low"/"medium"/"high") when no entry or map
// is found — safe to call for any harness/model combination.
func (c *Catalog) EffortFor(harness, modelID string, effort core.Effort) string {
	if entry, ok := c.EntryFor(harness, modelID); ok {
		return EffortValue(entry, effort)
	}
	return effort.String()
}

// AllowedEfforts implements core.EffortLister. It returns the effort levels the
// catalog declares for the exact model id (id, alias or native name). A family
// prefix match never counts: a list is only trusted for the model it was
// verified on. Nil means unknown.
func (c *Catalog) AllowedEfforts(harness, modelID string) []string {
	if c == nil || modelID == "" {
		return nil
	}
	if slug, ok := gatewaySlug(modelID); ok {
		if slug == "" {
			return nil
		}
		for _, e := range c.entries {
			if isGatewayProvider(e.Provider) && e.Harness == harness && isGatewayName(e, slug) {
				return e.Efforts
			}
		}
		return nil
	}
	for _, id := range uniqueStrings(modelID, normaliseModelID(modelID)) {
		for _, e := range c.entries {
			if e.Harness != harness || len(e.Efforts) == 0 {
				continue
			}
			if e.ID == id || e.NativeName == id {
				return e.Efforts
			}
			for _, a := range e.Aliases {
				if a == id {
					return e.Efforts
				}
			}
		}
	}
	return nil
}

// EffortUnknownModels lists "harness/id" for every entry that declares no
// efforts list, so doctor can show where effort values are not constrained.
func (c *Catalog) EffortUnknownModels() []string {
	var out []string
	for _, e := range c.entries {
		if e.ID != "" && len(e.Efforts) == 0 {
			out = append(out, e.Harness+"/"+e.ID)
		}
	}
	return out
}
