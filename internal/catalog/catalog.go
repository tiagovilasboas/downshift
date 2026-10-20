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
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	_ "embed"

	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/paths"
)

//go:embed catalog.json
var embeddedJSON []byte

// Entry is one row in the catalog JSON.
// ProviderOpenRouter marks a gateway row: a model reached through OpenRouter,
// known only by its full "vendor/model" slug.
const ProviderOpenRouter = "openrouter"

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
	InputCostM  float64           `json:"input_cost_per_1m"`
	OutputCostM float64           `json:"output_cost_per_1m"`
	// CacheReadCostM / CacheWriteCostM are optional list prices for cached
	// input tokens (USD per 1M). Zero means "not in the catalog": never a
	// real free price. Filled by scripts/sync_catalog_prices.py from a snapshot.
	CacheReadCostM  float64 `json:"cache_read_cost_per_1m"`
	CacheWriteCostM float64 `json:"cache_write_cost_per_1m"`
}

// catalogFile is the structure of the JSON file on disk.
type catalogFile struct {
	Version string `json:"version"`
	Updated string `json:"updated"`
	// PricesAsOf (YYYY-MM-DD) is when the list prices were last verified
	// against a public source. Only the embedded catalog's value is trusted:
	// a user override merges entries, not freshness.
	PricesAsOf string  `json:"prices_as_of"`
	Entries    []Entry `json:"entries"`
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
	// byFamEff: harness → family (lowercased) → effort (lowercased) → Model.
	// Only entries with an explicit effort tag that are not explicit_only are
	// indexed here. Powers FamilyModelFor (Option A: family + effort routing).
	byFamEff map[string]map[string]map[string]core.Model
	// entries: all raw entries (for list/check/pull commands)
	entries []Entry
	// pricesAsOf: catalogFile.PricesAsOf, empty when absent.
	pricesAsOf string
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
	return build(base)
}

// applyUserOverride merges the user catalog into base. A user catalog is an
// override, not a replacement: it preserves all embedded entries it does not
// name and replaces only matching harness/model IDs. The file is untrusted
// input read on every hook, so any failure, even a panic in the merge, leaves
// base untouched (embedded catalog) and only warns.
func applyUserOverride(base *catalogFile, warning io.Writer) {
	path, ok := userCatalogPath()
	if !ok {
		return
	}
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(warning, "downshift: warning: could not apply %s (%v); using embedded catalog\n", path, r)
		}
	}()
	data, err := readUserCatalog(path)
	if err != nil {
		fmt.Fprintf(warning, "downshift: warning: could not read %s (%v); using embedded catalog\n", path, err)
		return
	}
	override, err := decode(data)
	if err != nil {
		fmt.Fprintf(warning, "downshift: warning: could not parse %s (%v); using embedded catalog\n", path, err)
		return
	}
	merged := mergeEntries(base.Entries, override.Entries)
	if err := validateEntries(merged); err != nil {
		fmt.Fprintf(warning, "downshift: warning: could not merge %s (%v); using embedded catalog\n", path, err)
		return
	}
	base.Entries = merged
}

// maxUserCatalogBytes caps the user override read: the file is untrusted
// input on every hook invocation.
const maxUserCatalogBytes = 4 << 20

// readUserCatalog reads the override only if it is a regular file, and never
// more than maxUserCatalogBytes. A FIFO, device or huge file is an error (the
// caller falls back to the embedded catalog) instead of a hang or an OOM.
func readUserCatalog(path string) ([]byte, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("not a regular file")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxUserCatalogBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxUserCatalogBytes {
		return nil, fmt.Errorf("larger than %d bytes", maxUserCatalogBytes)
	}
	return data, nil
}

// userCatalogPath returns the path to the user override file and whether it
// exists. Returns false if XDG / home resolution fails.
func userCatalogPath() (string, bool) {
	p, err := paths.CatalogPath()
	if err != nil {
		return "", false
	}
	if _, err := os.Stat(p); err != nil {
		return "", false
	}
	return p, true
}

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
	return f, nil
}

func build(f catalogFile) *Catalog {
	c := &Catalog{
		index:     make(map[string]map[core.Tier]core.Model),
		harnesses: make(map[string]struct{}),
		byID:      make(map[string]map[string]core.Model),
		byFamily:  make(map[string][]familyEntry),
		byFamEff:  make(map[string]map[string]map[string]core.Model),
		entries:   f.Entries,

		pricesAsOf: f.PricesAsOf,
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
			ID:          e.ID,
			Family:      e.Family,
			Native:      e.NativeName,
			Tier:        tier,
			InputM:      e.InputCostM,
			OutputM:     e.OutputCostM,
			CacheReadM:  e.CacheReadCostM,
			CacheWriteM: e.CacheWriteCostM,
			Harness:     e.Harness,
		}

		if c.index[e.Harness] == nil {
			c.index[e.Harness] = make(map[core.Tier]core.Model)
		}
		// Explicit-only models are valid current selections, but never automatic targets.
		if e.Routing != "explicit_only" {
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
	}
	return c
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

// SavingsRatio returns how much cheaper `to` is versus `from` as a fraction.
func (c *Catalog) SavingsRatio(from, to core.Model) float64 {
	fromCost := from.InputM + from.OutputM
	toCost := to.InputM + to.OutputM
	if from.InputM <= 0 || from.OutputM <= 0 || to.InputM <= 0 || to.OutputM <= 0 || toCost >= fromCost {
		return 0
	}
	return (fromCost - toCost) / fromCost
}

// --- Accessors for commands (models list / check / pull) ---

// PricesAsOf returns the date (YYYY-MM-DD) the list prices were last verified,
// or "" when the catalog carries none.
func (c *Catalog) PricesAsOf() string {
	if c == nil {
		return ""
	}
	return c.pricesAsOf
}

// PricesAge returns how many whole days old the list prices are at now. ok is
// false when prices_as_of is missing or not a YYYY-MM-DD date.
func (c *Catalog) PricesAge(now time.Time) (days int, ok bool) {
	d, err := time.Parse("2006-01-02", c.PricesAsOf())
	if err != nil {
		return 0, false
	}
	return int(now.UTC().Sub(d).Hours() / 24), true
}

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
	return entry.Routing == "explicit_only"
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
