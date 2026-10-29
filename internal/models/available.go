// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package models

import (
	"errors"
	"strings"
	"time"

	"github.com/tiagovilasboas/downshift/internal/core"
)

// Where a model of the available list came from.
const (
	SourcePayload         = "payload"
	SourceDiscovery       = "discovery"
	SourceFile            = "file"
	SourceReported        = "mcp"
	SourceCatalogFallback = "catalog-fallback"
)

// TierUnknown is the tier of a model no source or catalog row ranks.
const TierUnknown = "unknown"

// ErrUnknownHarness is returned for a harness id core does not declare.
var ErrUnknownHarness = errors.New("unknown harness")

// Metadata is what the resolver reads from the catalog. The catalog gives tier
// and effort data for an id a real source listed; it never adds an id, except
// through SessionCandidates for a harness that declares SessionFromCatalog and
// whose real list cannot be learned.
type Metadata interface {
	LookupByID(harness, modelID string) (core.Model, bool)
	AllowedEfforts(harness, modelID string) []string
	SessionCandidates(harness string) []string
	IsExplicitOnly(harness, modelID string) bool
}

// Available is one model a spawn can use on a harness right now.
type Available struct {
	ID      string   `json:"id"`
	Tier    string   `json:"tier"`
	Efforts []string `json:"efforts,omitempty"`
	Source  string   `json:"source"`
	// Untiered is true when no evidence ranks the id. Such a model is listed,
	// never the target of a hint or a rule; the orchestrator may name it.
	Untiered bool `json:"untiered"`
}

// Listing is the answer for one harness.
type Listing struct {
	Harness string      `json:"harness"`
	Models  []Available `json:"models"`
	// Fallback is true when only the catalog stand-in exists.
	Fallback bool `json:"fallback"`
	// CacheWrittenAt is when the discovery cache entry was written; zero when
	// the list did not come from discovery.
	CacheWrittenAt time.Time `json:"cache_written_at,omitzero"`
}

// Options carry the per-call inputs of Resolve.
type Options struct {
	// Payload is the model list a hook payload carried. Nil means the payload
	// had none; a non-nil slice is authoritative, even when empty.
	Payload *[]string
	// Now is the clock for discovery expiry; zero means time.Now.
	Now time.Time
}

// Resolve answers which models a spawn can use on harness. Sources are tried
// in this order and the first that lists models wins, like session
// resolution: payload, discovery cache, session-models.json, then the catalog
// stand-in. No list at all is an empty Models slice, never a guess.
func Resolve(harness string, md Metadata, opt Options) (Listing, error) {
	caps, ok := core.CapsFor(harness)
	if !ok {
		return Listing{}, ErrUnknownHarness
	}
	if opt.Now.IsZero() {
		opt.Now = time.Now()
	}
	out := Listing{Harness: harness, Models: []Available{}}
	src, found := readSource(harness, opt)
	if !found && caps.SessionFromCatalog {
		out.Models = fallbackModels(harness, md)
		out.Fallback = len(out.Models) > 0
		return out, nil
	}
	if !found {
		return out, nil
	}
	out.CacheWrittenAt = src.writtenAt
	for _, id := range src.ids {
		out.Models = append(out.Models, describe(harness, id, src.source, src.meta[id], md))
	}
	for _, id := range src.unranked {
		out.Models = append(out.Models, Available{ID: id, Tier: TierUnknown, Source: src.source, Untiered: true})
	}
	return out, nil
}

// fallbackModels is the Claude Code stand-in: the catalog's session
// candidates that resolve to an automatic model.
func fallbackModels(harness string, md Metadata) []Available {
	var out []Available
	seen := map[string]bool{}
	for _, name := range md.SessionCandidates(harness) {
		m, ok := md.LookupByID(harness, name)
		if !ok || m.ID == "" || seen[m.ID] || md.IsExplicitOnly(harness, m.ID) {
			continue
		}
		seen[m.ID] = true
		out = append(out, describe(harness, m.ID, SourceCatalogFallback, nil, md))
	}
	return out
}

// describe builds the entry for an id a source listed. Source metadata wins
// over the catalog; the catalog only fills what the source did not say.
func describe(harness, id, source string, evidence *evidenceRow, md Metadata) Available {
	a := Available{ID: id, Tier: TierUnknown, Source: source}
	if evidence != nil {
		a.Tier = normalizeTier(evidence.tier)
		a.Efforts = append([]string(nil), evidence.efforts...)
	}
	if a.Tier == TierUnknown {
		if m, ok := md.LookupByID(harness, id); ok {
			a.Tier = m.Tier.String()
		}
	}
	if len(a.Efforts) == 0 {
		a.Efforts = md.AllowedEfforts(harness, id)
	}
	a.Untiered = a.Tier == TierUnknown
	return a
}

func normalizeTier(s string) string {
	switch t := strings.ToLower(strings.TrimSpace(s)); t {
	case "small", "mid", "frontier":
		return t
	}
	return TierUnknown
}
