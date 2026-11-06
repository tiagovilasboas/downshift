// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package catalog owns all model-ID data for Downshift.
//
// core never imports catalog. The dependency runs one way:
//
//	catalog → core (for Tier, Effort types)
//	adapters → catalog (for a Resolver at startup)
//
// Model data lives in catalog.json (embedded at compile time). Users may
// override it by placing a file at ~/.harness-downshift/catalog.json — the
// override is merged at load time, with the user file taking precedence.
// Any error reading the user file is logged to stderr and silently ignored;
// the embedded catalog is always the safe fallback.
package catalog

import (
	"io"
	"os"
	"strings"

	_ "embed"

	"github.com/tiagovilasboas/downshift/internal/core"
)

//go:embed catalog.json
var embeddedJSON []byte

// isGatewayProvider reports whether a catalog row's provider is a recognized
// gateway (OpenRouter or 9router).
func isGatewayProvider(provider string) bool {
	return provider != "" && (provider == AllowlistProvider || provider == "9router")
}

// gatewaySlug splits the OpenCode form of a gateway model,
// "<gateway>/<vendor>/<model>", when the first segment names a registered
// gateway. Such an id resolves only to gateway rows.
func gatewaySlug(id string) (string, bool) {
	name, slug, ok := strings.Cut(id, "/")
	if !ok || !isGatewayProvider(name) {
		return "", false
	}
	return slug, true
}

// RoutingShadow marks an allowlisted gateway row. It is never an automatic
// routing target and never a current-model write target; it is only a
// candidate the shadow would_route field may name (see ShadowGateways).
const RoutingShadow = "shadow"

// RoutingLive marks an allowlisted gateway row the user made live. It is an
// explicit target for the rewrite of its own harness and tier (see
// LiveGateways); it is still never indexed as a native automatic target.
const RoutingLive = "live"

// Entry is one row in the catalog JSON.
type Entry struct {
	ID          string            `json:"id"`
	Aliases     []string          `json:"aliases"`
	Family      string            `json:"family"`      // stable prefix for version-agnostic matching
	NativeName  string            `json:"native_name"` // optional: name the harness schema accepts instead of the id (family-level, survives version bumps)
	Effort      string            `json:"effort"`      // optional: low|medium|high — effort variant inside the family (Option A)
	Provider    string            `json:"provider"`
	Harness     string            `json:"harness"`
	Tier        string            `json:"tier"`    // "small" | "mid" | "frontier" | "unknown"
	Routing     string            `json:"routing"` // "automatic" (default) | "explicit_only"
	EffortScale string            `json:"effort_scale"`
	EffortMap   map[string]string `json:"effort_map"` // "low"|"medium"|"high" → native string
	Efforts     []string          `json:"efforts"`    // optional: effort levels the model accepts; absent = unknown, no restriction
	// PoolGated marks a model whose rewrite needs positive token-pool evidence
	// on the call (see core.Model.PoolGated).
	PoolGated bool `json:"pool_gated"`
	// RejectedName is an alias of this model the harness's spawn tool refuses;
	// the id is written in its place (see core.Model.RejectedName).
	RejectedName string `json:"rejected_name"`
	// ZDR marks a gateway row whose user allowlist says the provider keeps no
	// data. Carried only; refusing a non-ZDR row for a risky task is not done yet.
	ZDR bool `json:"zdr"`
	// Old catalogs (embedded and user overrides) carried per-1M list prices
	// (input_cost_per_1m and the rest) and a top-level prices_as_of. The
	// product no longer uses a price; the decoder is not strict, so a user
	// catalog that still has those keys loads and they are ignored.
}

// catalogFile is the structure of the JSON file on disk.
type catalogFile struct {
	Version string  `json:"version"`
	Updated string  `json:"updated"`
	Entries []Entry `json:"entries"`
	// SessionCandidates maps a harness to the model names it accepts in any
	// session, least to most capable (see core.CatalogSessionFromCurrent).
	SessionCandidates map[string][]string `json:"session_candidates"`
}

// Catalog is the resolved in-memory model store, implementing core.Resolver.
type Catalog struct {
	// index: harness → tier → Model (primary lookup for routing)
	index map[string]map[core.Tier]core.Model
	// harnesses records every harness declared by the catalog, including one
	// whose current entries do not provide every routable tier. A declared
	// harness must never borrow a model ID from another harness.
	harnesses map[string]struct{}
	// byID: harness → modelID/alias → Model (exact and alias lookup)
	byID map[string]map[string]core.Model
	// byFamily: harness → family prefix → Model (version-agnostic fallback)
	byFamily map[string][]familyEntry
	// gateway: harness → gateway row id/alias → Model. Only OpenRouter rows
	// are here; the "openrouter/<slug>" form resolves through these two
	// gateway indexes and never through byID or byFamily.
	gateway map[string]map[string]core.Model
	// gatewayFamily: harness → family prefix of the gateway rows.
	gatewayFamily map[string][]familyEntry
	// byFamEff: harness → family (lowercased) → effort (lowercased) → Model.
	// Only entries with an explicit effort tag that are not explicit_only are
	// indexed here. Powers FamilyModelFor (Option A: family + effort routing).
	byFamEff map[string]map[string]map[string]core.Model
	// entries: all raw entries (for list/check/pull commands)
	entries []Entry
	// sessionCandidates: catalogFile.SessionCandidates.
	sessionCandidates map[string][]string
}

// familyEntry pairs a stable family prefix with the model it maps to.
type familyEntry struct {
	prefix string
	model  core.Model
}

// familyEffortTag pairs a family prefix with its effort variant tag for
// overlap validation: same-family entries with differing efforts are effort
// variants (Option A), not configuration errors.
type familyEffortTag struct {
	prefix string
	effort string // normalised low|medium|high, or "" when untagged
}

// Load returns the effective Catalog. Load priority:
//  1. ~/.harness-downshift/catalog.json (user override, if present and valid)
//  2. Embedded catalog.json (compile-time default, always valid)
//
// If the user file exists but cannot be parsed, a warning is printed to
// stderr and Load falls back to the embedded file — fail-open, always.
func Load() *Catalog {
	return load(os.Stderr)
}

// load resolves the embedded catalog and optional user override. warning is
// injected for tests; production callers use stderr through Load.
func load(warning io.Writer) *Catalog {
	base, err := decode(embeddedJSON)
	if err != nil {
		panic("downshift: embedded catalog.json is invalid: " + err.Error())
	}

	applyUserOverride(&base, warning)
	applyOpenRouterAllowlist(&base, warning)
	return build(base)
}
