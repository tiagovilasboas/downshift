// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package catalog

import (
	"encoding/json"
	"strings"

	"github.com/tiagovilasboas/downshift-ai/internal/core"
)

// parse decodes a catalog JSON payload into a Catalog.
func parse(data []byte) (*Catalog, error) {
	f, err := decode(data)
	if err != nil {
		return nil, err
	}
	return build(f), nil
}

func decode(data []byte) (catalogFile, error) {
	var f catalogFile
	if err := json.Unmarshal(data, &f); err != nil {
		return catalogFile{}, err
	}
	if err := validateEntries(f.Entries); err != nil {
		return catalogFile{}, err
	}
	for i := range f.Entries {
		if isGatewayProvider(f.Entries[i].Provider) && f.Entries[i].Routing == "" {
			f.Entries[i].Routing = "explicit_only"
		}
	}
	return f, nil
}

func build(f catalogFile) *Catalog {
	c := &Catalog{
		index:         make(map[string]map[core.Tier]core.Model),
		harnesses:     make(map[string]struct{}),
		byID:          make(map[string]map[string]core.Model),
		byFamily:      make(map[string][]familyEntry),
		gateway:       make(map[string]map[string]core.Model),
		gatewayFamily: make(map[string][]familyEntry),
		byFamEff:      make(map[string]map[string]map[string]core.Model),
		entries:       f.Entries,

		sessionCandidates: f.SessionCandidates,
	}

	for _, e := range f.Entries {
		if e.Harness != "" {
			c.harnesses[e.Harness] = struct{}{}
		}
		tier, ok := parseTier(e.Tier)
		if !ok || e.ID == "" || e.Harness == "" {
			continue
		}
		m := core.Model{
			ID:      e.ID,
			Family:  e.Family,
			Native:  e.NativeName,
			Tier:    tier,
			Harness: e.Harness,

			PoolGated:    e.PoolGated,
			RejectedName: e.RejectedName,
		}

		if c.index[e.Harness] == nil {
			c.index[e.Harness] = make(map[core.Tier]core.Model)
		}
		// Explicit-only models are valid current selections, but never automatic targets.
		if automaticRoutable(e) {
			if _, exists := c.index[e.Harness][tier]; !exists {
				c.index[e.Harness][tier] = m
			}
			// Effort-tagged entries feed the family+effort index (Option A).
			// Explicit-only entries are skipped here as well: they must never
			// be chosen automatically, only preserved when already selected.
			if eff := normalizeEffort(e.Effort); eff != "" && e.Family != "" {
				fam := strings.ToLower(e.Family)
				if c.byFamEff[e.Harness] == nil {
					c.byFamEff[e.Harness] = make(map[string]map[string]core.Model)
				}
				if c.byFamEff[e.Harness][fam] == nil {
					c.byFamEff[e.Harness][fam] = make(map[string]core.Model)
				}
				if _, exists := c.byFamEff[e.Harness][fam][eff]; !exists {
					c.byFamEff[e.Harness][fam][eff] = m
				}
			}
		}

		if c.byID[e.Harness] == nil {
			c.byID[e.Harness] = make(map[string]core.Model)
		}
		c.byID[e.Harness][e.ID] = m
		for _, alias := range e.Aliases {
			c.byID[e.Harness][alias] = m
		}
		// The harness sends its native name back in tool input (Claude Code's
		// Task schema accepts "haiku"/"sonnet"/"opus"/"fable"), so it must
		// resolve like an alias. The first entry of a family keeps the name.
		if e.NativeName != "" {
			if _, taken := c.byID[e.Harness][e.NativeName]; !taken {
				c.byID[e.Harness][e.NativeName] = m
			}
		}

		// Build the family index for version-agnostic fallback.
		// Each family prefix is added once per (harness, family) pair.
		if e.Family != "" {
			fe := familyEntry{prefix: strings.ToLower(e.Family), model: m}
			c.byFamily[e.Harness] = append(c.byFamily[e.Harness], fe)
		}

		if isGatewayProvider(e.Provider) {
			indexGateway(c, e, m)
		}
	}
	return c
}

// indexGateway adds one OpenRouter row to the gateway indexes.
func indexGateway(c *Catalog, e Entry, m core.Model) {
	if c.gateway[e.Harness] == nil {
		c.gateway[e.Harness] = make(map[string]core.Model)
	}
	c.gateway[e.Harness][e.ID] = m
	for _, alias := range e.Aliases {
		c.gateway[e.Harness][alias] = m
	}
	if e.Family != "" {
		fe := familyEntry{prefix: strings.ToLower(e.Family), model: m}
		c.gatewayFamily[e.Harness] = append(c.gatewayFamily[e.Harness], fe)
	}
}

// parseTier converts the JSON string tier to a core.Tier constant.
func parseTier(s string) (core.Tier, bool) {
	switch strings.ToLower(s) {
	case "small":
		return core.TierSmall, true
	case "mid":
		return core.TierMid, true
	case "frontier":
		return core.TierFrontier, true
	default:
		return 0, false
	}
}

// normalizeEffort maps the JSON effort tag to its canonical lowercase form.
// Returns "" for empty or unrecognised values; untagged entries keep the
// legacy tier-only behaviour and never enter the family+effort index.
func normalizeEffort(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "low":
		return "low"
	case "medium":
		return "medium"
	case "high":
		return "high"
	default:
		return ""
	}
}
