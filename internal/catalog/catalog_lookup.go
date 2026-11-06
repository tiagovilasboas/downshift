// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package catalog

import (
	"strings"

	"github.com/tiagovilasboas/downshift/internal/core"
)

// --- core.Resolver implementation ---

// ModelFor returns the model for the given harness and tier.
//
// A declared harness that lacks a model for the requested tier returns a zero
// model for that same harness. This prevents an incomplete per-harness catalog
// from sending a model ID that belongs to another tool. Only an entirely
// unknown harness falls back to the claude-code catalog.
func (c *Catalog) ModelFor(harness string, tier core.Tier) core.Model {
	if h, ok := c.index[harness]; ok {
		if m, ok := h[tier]; ok {
			return m
		}
	}
	if _, known := c.harnesses[harness]; known {
		return core.Model{Tier: tier, Harness: harness}
	}
	// Fallback: claude-code is always present in the embedded catalog.
	if h, ok := c.index["claude-code"]; ok {
		if m, ok := h[tier]; ok {
			return m
		}
	}
	// Last resort: zero model (should never happen with embedded catalog).
	return core.Model{Tier: tier, Harness: harness}
}

// FamilyModelFor implements core.FamilyEffortResolver: it returns the
// automatic routing target for one model family at one effort level,
// without versioning the choice per harness (Option A).
//
// Lookup order: exact family key first, then longest-prefix fallback across
// effort-tagged families of the same harness (keeps version-agnostic
// behaviour for bumped IDs). Explicit-only entries are never indexed here,
// so they are never returned. Unknown family or effort returns false and
// the caller falls back to ModelFor.
func (c *Catalog) FamilyModelFor(harness, family string, effort core.Effort) (core.Model, bool) {
	fam := strings.ToLower(strings.TrimSpace(family))
	if fam == "" {
		return core.Model{}, false
	}
	eff := strings.ToLower(effort.String())
	fams, ok := c.byFamEff[harness]
	if !ok {
		return core.Model{}, false
	}
	if variants, ok := fams[fam]; ok {
		if m, ok := variants[eff]; ok {
			return m, true
		}
		return core.Model{}, false
	}
	// Longest-prefix fallback: a bumped family ID still lands on its base
	// family (e.g. "claude-opus-5" → "claude-opus") when that family has an
	// effort variant.
	best := ""
	for prefix := range fams {
		if strings.HasPrefix(fam, prefix) && len(prefix) > len(best) {
			best = prefix
		}
	}
	if best == "" {
		return core.Model{}, false
	}
	m, ok := fams[best][eff]
	return m, ok
}

// IsExactID reports an exact catalog id or a declared alias for this harness.
// Family-prefix matches are not exact. core uses this so a foreign harness
// version string is not treated as local ownership.
func (c *Catalog) IsExactID(harness, id string) bool {
	if c == nil || harness == "" || id == "" {
		return false
	}
	if slug, ok := gatewaySlug(id); ok {
		_, exact := c.gateway[harness][slug]
		return exact && slug != ""
	}
	h, ok := c.byID[harness]
	if !ok {
		return false
	}
	if _, ok := h[id]; ok {
		return true
	}
	_, ok = h[normaliseModelID(id)]
	return ok
}

// LookupByID resolves a model ID to a catalog entry using three strategies,
// in priority order:
//
//  1. Exact match — the full ID matches (e.g. "claude-opus-4-8").
//  2. Alias match — the ID matches a declared alias (e.g. "claude-opus").
//  3. Family prefix match — the ID starts with the entry's family string
//     (e.g. "claude-opus-4-9" matches family "claude-opus"). Case-insensitive.
//
// OpenRouter model IDs arrive in "provider/model-id" format
// (e.g. "anthropic/claude-opus-4-8", "openai/gpt-5.3-codex"). LookupByID
// strips the provider prefix before matching, so OpenRouter works
// transparently without any special configuration from the user.
//
// This makes routing version-agnostic: a new model version is automatically
// mapped to the same tier as the previous version in the same family, without
// any catalog update.
func (c *Catalog) LookupByID(harness, modelID string) (core.Model, bool) {
	if modelID == "" {
		return core.Model{}, false
	}
	if slug, ok := gatewaySlug(modelID); ok {
		return c.lookupGateway(harness, slug)
	}

	// Normalise: strip the "provider/" prefix used by OpenRouter and similar
	// meta-providers (e.g. "anthropic/claude-opus-4-8" → "claude-opus-4-8").
	normalised := normaliseModelID(modelID)

	// 1 & 2: exact / alias (already merged in byID map, O(1)).
	// Try both the original ID and the normalised form.
	for _, id := range uniqueStrings(modelID, normalised) {
		if h, ok := c.byID[harness]; ok {
			if m, ok := h[id]; ok {
				return m, true
			}
		}
	}

	// 3: family prefix — version-agnostic fallback. The full slug is tried
	// before the stripped form, so a gateway row whose family carries its own
	// "vendor/" prefix wins over a native family that only matches the tail.
	for _, lower := range familyCandidates(modelID) {
		for _, fe := range c.byFamily[harness] {
			if strings.HasPrefix(lower, fe.prefix) {
				return fe.model, true
			}
		}
	}

	return core.Model{}, false
}

// lookupGateway resolves the slug of an "openrouter/<slug>" id against the
// gateway rows only: exact slug or alias first, then the family prefix.
func (c *Catalog) lookupGateway(harness, slug string) (core.Model, bool) {
	if slug == "" {
		return core.Model{}, false
	}
	if m, ok := c.gateway[harness][slug]; ok {
		return m, true
	}
	lower := strings.ToLower(slug)
	for _, fe := range c.gatewayFamily[harness] {
		if strings.HasPrefix(lower, fe.prefix) {
			return fe.model, true
		}
	}
	return core.Model{}, false
}

// familyCandidates lists the lower-case forms of an id to try against family
// prefixes: the full id first, then the form with its first "provider/"
// segment stripped. An id with no slash has one candidate.
func familyCandidates(modelID string) []string {
	return uniqueStrings(strings.ToLower(modelID), strings.ToLower(normaliseModelID(modelID)))
}

// normaliseModelID strips the "provider/" prefix from OpenRouter-style model
// IDs. "anthropic/claude-opus-4-8" → "claude-opus-4-8". IDs without a slash
// are returned unchanged.
func normaliseModelID(id string) string {
	if i := strings.Index(id, "/"); i >= 0 {
		return id[i+1:]
	}
	return id
}

// uniqueStrings returns a deduplicated slice preserving order.
func uniqueStrings(a, b string) []string {
	if a == b {
		return []string{a}
	}
	return []string{a, b}
}
