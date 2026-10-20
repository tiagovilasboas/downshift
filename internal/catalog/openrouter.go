// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package catalog

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode"
)

// AllowlistProvider is the gateway the user allowlist file belongs to. Its rows
// carry this provider, so the registry must hold a gateway of this name.
const AllowlistProvider = "openrouter"

// openRouterAllowlistHarness is the only harness the allowlist may target.
const openRouterAllowlistHarness = "opencode"

// openRouterAllowlistFile is the schema of the user's openrouter.json:
//
//	{"version":1,"harness":"opencode","models":[
//	  {"id":"vendor/model","tier":"small|mid|frontier","max_effort":"low|medium|high","zdr":true}]}
type openRouterAllowlistFile struct {
	Version int                      `json:"version"`
	Harness string                   `json:"harness"`
	Models  []openRouterAllowlistRow `json:"models"`
}

type openRouterAllowlistRow struct {
	ID        string `json:"id"`
	Tier      string `json:"tier"`
	MaxEffort string `json:"max_effort"`
	ZDR       bool   `json:"zdr"`
}

// parseOpenRouterAllowlist validates the whole file and returns one shadow
// gateway row per listed model. Any violation rejects the file: no row is
// returned, so nothing is applied partially. The rows carry no price; the
// catalog never invents one for a gateway model.
func parseOpenRouterAllowlist(data []byte) ([]Entry, error) {
	var f openRouterAllowlistFile
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}
	if f.Version != 1 {
		return nil, fmt.Errorf("version must be 1, got %d", f.Version)
	}
	if f.Harness != openRouterAllowlistHarness {
		return nil, fmt.Errorf("harness must be %q, got %q", openRouterAllowlistHarness, f.Harness)
	}

	seen := make(map[string]bool, len(f.Models))
	rows := make([]Entry, 0, len(f.Models))
	for _, m := range f.Models {
		if !isFullSlug(m.ID) {
			return nil, fmt.Errorf("model id %q must be the full vendor/model slug", m.ID)
		}
		key := strings.ToLower(m.ID)
		if seen[key] {
			return nil, fmt.Errorf("duplicate model id %q", m.ID)
		}
		seen[key] = true

		if _, ok := parseTier(m.Tier); !ok {
			return nil, fmt.Errorf("model %q: unknown tier %q (want small, mid or frontier)", m.ID, m.Tier)
		}
		efforts, ok := effortsUpTo(m.MaxEffort)
		if !ok {
			return nil, fmt.Errorf("model %q: unknown max_effort %q (want low, medium or high)", m.ID, m.MaxEffort)
		}

		rows = append(rows, Entry{
			ID:       m.ID,
			Provider: AllowlistProvider,
			Harness:  openRouterAllowlistHarness,
			Tier:     strings.ToLower(m.Tier),
			Routing:  RoutingShadow,
			Efforts:  efforts,
			ZDR:      m.ZDR,
		})
	}
	return rows, nil
}

// isFullSlug reports a "vendor/model" id with both parts non-empty and no
// control character or whitespace anywhere in it.
func isFullSlug(id string) bool {
	vendor, model, ok := strings.Cut(id, "/")
	return ok && vendor != "" && model != "" && !strings.Contains(model, "/") && !hasControlOrSpace(id)
}

// hasControlOrSpace reports whether s holds a control character or whitespace,
// such as a newline, which must never reach a model id.
func hasControlOrSpace(s string) bool {
	for _, r := range s {
		if unicode.IsControl(r) || unicode.IsSpace(r) {
			return true
		}
	}
	return false
}

// effortsUpTo turns a max_effort cap into the list of effort levels the model
// accepts: the cap and every level below it.
func effortsUpTo(maxEffort string) ([]string, bool) {
	switch normalizeEffort(maxEffort) {
	case "low":
		return []string{"low"}, true
	case "medium":
		return []string{"low", "medium"}, true
	case "high":
		return []string{"low", "medium", "high"}, true
	default:
		return nil, false
	}
}
