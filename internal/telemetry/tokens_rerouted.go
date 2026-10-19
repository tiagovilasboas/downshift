// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"sort"

	"github.com/tiagovilasboas/downshift/internal/core"
)

// Tokens rerouted: the token facts behind a saving.
//
// Downshift does not reduce tokens. A routed child runs about the same tokens
// on another model; the dollar saving is those same tokens priced on the model
// the harness asked for (a counterfactual). So the figures here are counts of
// tokens that ran on a different model than the one requested, never "tokens
// saved". Counts only: no prompt, task, path, session or agent id.

// TokensBasis labels every figure of TokensRerouted.
const TokensBasis = "tokens as reported by the harness; dollars elsewhere are the same tokens priced on the requested model, an estimate"

// Where a priced usage record's tokens land.
const (
	// TokenCheaper: an applied rewrite whose model cost less than the requested one.
	TokenCheaper = "cheaper"
	// TokenPricier: an applied rewrite whose model cost more (an upshift).
	TokenPricier = "pricier"
	// TokenUnchanged: the child ran on the requested model, or at the same
	// price: an allow, a held or blocked decision, the control group, a
	// rewrite the harness did not honor, or an applied rewrite priced the same.
	TokenUnchanged = "unchanged"
	// TokenUnattributed: the record cannot be placed: no linked decision in
	// this window, or an applied rewrite whose requested model has no price.
	TokenUnattributed = "unattributed"
)

// Harness token status. "unobserved" is a harness with no token source and no
// record: never a measured zero.
const (
	TokenStatusObserved    = "observed"
	TokenStatusNoneInRange = "none_in_window"
	TokenStatusUnobserved  = "unobserved"
)

// TokenClasses is a token count split the way providers bill it. Cache read is
// the cached total minus the cache-creation (write) part.
type TokenClasses struct {
	Records    int   `json:"records"`
	Input      int64 `json:"input"`
	Output     int64 `json:"output"`
	CacheRead  int64 `json:"cache_read"`
	CacheWrite int64 `json:"cache_write"`
	// Total is Input + Output + CacheRead + CacheWrite.
	Total int64 `json:"total"`
}

// classesOf splits a usage into billing classes; the split mirrors CostUSD.
func classesOf(u TokenUsage) TokenClasses {
	cached := max(u.CachedTokens, 0)
	write := min(max(u.CacheWriteTokens, 0), cached)
	c := TokenClasses{
		Input:      max(u.InputTokens, 0),
		Output:     max(u.OutputTokens, 0),
		CacheRead:  cached - write,
		CacheWrite: write,
	}
	c.Total = c.Input + c.Output + c.CacheRead + c.CacheWrite
	return c
}

// Cache is cache read plus cache write.
func (c TokenClasses) Cache() int64 { return c.CacheRead + c.CacheWrite }

func (c *TokenClasses) add(o TokenClasses) {
	c.Records += o.Records
	c.Input += o.Input
	c.Output += o.Output
	c.CacheRead += o.CacheRead
	c.CacheWrite += o.CacheWrite
	c.Total += o.Total
}

// TokenPair is the tokens that moved from one requested model to the model that
// ran, in one direction. Model ids are the bounded ids already in the log.
type TokenPair struct {
	Requested string `json:"requested_model"`
	Ran       string `json:"ran_model"`
	Direction string `json:"direction"`
	TokenClasses
}

// ModelTokens is one model's share of a harness's priced usage tokens.
type ModelTokens struct {
	Model string `json:"model"`
	TokenClasses
}

// HarnessTokens is one harness's token picture.
type HarnessTokens struct {
	// UsageSource is the harness's declared token seam: payload, transcript or
	// none (core.HarnessCapabilities.UsageSource).
	UsageSource string `json:"usage_source"`
	// Status is observed, none_in_window (a token seam exists, no priced
	// record in the window) or unobserved (no seam and no record).
	Status string `json:"status"`
	// The four buckets partition the harness's priced usage tokens. They are
	// nil unless Status is observed, so an unobserved harness shows no zero.
	Cheaper      *TokenClasses `json:"cheaper,omitempty"`
	Pricier      *TokenClasses `json:"pricier,omitempty"`
	Unchanged    *TokenClasses `json:"unchanged,omitempty"`
	Unattributed *TokenClasses `json:"unattributed,omitempty"`
	// ByPair lists cheaper, pricier and unchanged tokens per requested -> ran pair.
	ByPair []TokenPair `json:"by_pair,omitempty"`
	// ByModel is the token mix: every priced usage record by the model that ran.
	ByModel []ModelTokens `json:"by_model,omitempty"`
}

// TokenTotals sums the buckets over the observed harnesses.
type TokenTotals struct {
	Cheaper      TokenClasses `json:"cheaper"`
	Pricier      TokenClasses `json:"pricier"`
	Unchanged    TokenClasses `json:"unchanged"`
	Unattributed TokenClasses `json:"unattributed"`
}

// TokensRerouted is the token view of routing, per harness.
type TokensRerouted struct {
	Basis     string                   `json:"basis"`
	ByHarness map[string]HarnessTokens `json:"by_harness"`
	// Totals is nil when no harness is observed.
	Totals *TokenTotals `json:"totals,omitempty"`
}

// HarnessOrder lists the harness ids in print order: declared harnesses first,
// in core.AllHarnessCaps order, then any other id seen in the log, sorted.
func (t TokensRerouted) HarnessOrder() []string {
	seen := map[string]bool{}
	var out []string
	for _, n := range core.AllHarnessCaps() {
		if _, ok := t.ByHarness[n.Harness]; ok {
			out = append(out, n.Harness)
			seen[n.Harness] = true
		}
	}
	var rest []string
	for h := range t.ByHarness {
		if !seen[h] {
			rest = append(rest, h)
		}
	}
	sort.Strings(rest)
	return append(out, rest...)
}

// tokenDirection places one priced usage record. decision is the linked
// decision (found); unrouted marks a linked decision the router left alone on
// purpose (control group, DOWNSHIFT_NO_ROUTE, pause); refuted marks an applied
// rewrite a resolved record says the harness did not run.
func tokenDirection(usage Event, decision Event, found, unrouted, refuted bool) string {
	switch {
	case unrouted:
		return TokenUnchanged
	case !found:
		return TokenUnattributed
	case !AppliedRewrite(decision.Outcome, decision.Corrections), refuted:
		return TokenUnchanged
	case !usage.HasRealCost():
		return TokenUnattributed
	}
	switch delta := usage.RealDeltaUSD(); {
	case delta > 0:
		return TokenCheaper
	case delta < 0:
		return TokenPricier
	}
	return TokenUnchanged
}

type pairKey struct {
	requested, ran, direction string
}

// harnessTokenAcc accumulates one harness while Aggregate runs.
type harnessTokenAcc struct {
	buckets map[string]*TokenClasses
	pairs   map[pairKey]*TokenClasses
	models  map[string]*TokenClasses
	records int
}

func newHarnessTokenAcc() *harnessTokenAcc {
	return &harnessTokenAcc{
		buckets: map[string]*TokenClasses{},
		pairs:   map[pairKey]*TokenClasses{},
		models:  map[string]*TokenClasses{},
	}
}

func accSlot[K comparable](m map[K]*TokenClasses, k K) *TokenClasses {
	c := m[k]
	if c == nil {
		c = &TokenClasses{}
		m[k] = c
	}
	return c
}

func knownModel(id string) bool { return id != "" && id != "unknown" }

// tokenModels returns the requested and ran model of a usage record. The record
// is the observation, so it wins; the linked decision fills a missing id. A
// rewrite the harness did not honor ran on the requested model.
func tokenModels(usage, decision Event, found, refuted bool) (requested, ran string) {
	requested, ran = usage.FromModel, usage.ToModel
	if !knownModel(requested) && found {
		requested = decision.FromModel
	}
	if refuted {
		return requested, requested
	}
	if !knownModel(ran) && found {
		if AppliedRewrite(decision.Outcome, decision.Corrections) {
			ran = decision.ToModel
		} else {
			ran = decision.FromModel
		}
	}
	return requested, ran
}

// countTokens folds one priced usage record into its harness. A record with no
// price or no tokens is not a priced token observation and adds nothing.
func (a *aggregator) countTokens(ev Event, decision Event, found, unrouted, refuted bool) {
	if ev.ActualCostUSD == nil {
		return
	}
	c := classesOf(ev.TokenUsageOrZero())
	if c.Total == 0 {
		return
	}
	c.Records = 1
	acc := a.tokens[ev.Harness]
	if acc == nil {
		acc = newHarnessTokenAcc()
		a.tokens[ev.Harness] = acc
	}
	acc.records++
	dir := tokenDirection(ev, decision, found, unrouted, refuted)
	accSlot(acc.buckets, dir).add(c)
	requested, ran := tokenModels(ev, decision, found, refuted)
	accSlot(acc.models, ran).add(c)
	if dir != TokenUnattributed {
		accSlot(acc.pairs, pairKey{requested, ran, dir}).add(c)
	}
}

var tokenDirectionOrder = map[string]int{TokenCheaper: 0, TokenPricier: 1, TokenUnchanged: 2}

// finalizeTokens builds the per-harness view. Every harness declared in
// core.AllHarnessCaps appears, whether or not it has records, so a harness with
// no token source reads "unobserved" instead of vanishing or showing zeros.
func (a *aggregator) finalizeTokens() TokensRerouted {
	out := TokensRerouted{Basis: TokensBasis, ByHarness: map[string]HarnessTokens{}}
	sources := map[string]string{}
	for _, n := range core.AllHarnessCaps() {
		src := n.Caps.UsageSource
		if src == "" {
			src = core.UsageNone
		}
		sources[n.Harness] = src
	}
	for h := range a.tokens {
		if _, ok := sources[h]; !ok {
			sources[h] = "unknown"
		}
	}
	var totals TokenTotals
	anyObserved := false
	for h, src := range sources {
		acc := a.tokens[h]
		ht := HarnessTokens{UsageSource: src}
		switch {
		case acc != nil && acc.records > 0:
			ht.Status = TokenStatusObserved
		case src == core.UsageNone:
			ht.Status = TokenStatusUnobserved
		default:
			ht.Status = TokenStatusNoneInRange
		}
		if ht.Status == TokenStatusObserved {
			anyObserved = true
			bucket := func(dir string) *TokenClasses {
				c := TokenClasses{}
				if b := acc.buckets[dir]; b != nil {
					c = *b
				}
				return &c
			}
			ht.Cheaper, ht.Pricier = bucket(TokenCheaper), bucket(TokenPricier)
			ht.Unchanged, ht.Unattributed = bucket(TokenUnchanged), bucket(TokenUnattributed)
			totals.Cheaper.add(*ht.Cheaper)
			totals.Pricier.add(*ht.Pricier)
			totals.Unchanged.add(*ht.Unchanged)
			totals.Unattributed.add(*ht.Unattributed)
			ht.ByPair = sortedPairs(acc.pairs)
			ht.ByModel = sortedModels(acc.models)
		}
		out.ByHarness[h] = ht
	}
	if anyObserved {
		out.Totals = &totals
	}
	return out
}

func sortedPairs(m map[pairKey]*TokenClasses) []TokenPair {
	out := make([]TokenPair, 0, len(m))
	for k, c := range m {
		out = append(out, TokenPair{Requested: k.requested, Ran: k.ran, Direction: k.direction, TokenClasses: *c})
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if da, db := tokenDirectionOrder[a.Direction], tokenDirectionOrder[b.Direction]; da != db {
			return da < db
		}
		if a.Total != b.Total {
			return a.Total > b.Total
		}
		if a.Requested != b.Requested {
			return a.Requested < b.Requested
		}
		return a.Ran < b.Ran
	})
	return out
}

func sortedModels(m map[string]*TokenClasses) []ModelTokens {
	out := make([]ModelTokens, 0, len(m))
	for model, c := range m {
		out = append(out, ModelTokens{Model: model, TokenClasses: *c})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Total != out[j].Total {
			return out[i].Total > out[j].Total
		}
		return out[i].Model < out[j].Model
	})
	return out
}
