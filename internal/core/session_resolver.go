// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"math"
	"regexp"
)

// SessionModel describes one exact selectable ID. Absent tier and prices mean
// unknown; omission must never imply small capability or free inference.
type SessionModel struct {
	ID      string   `json:"id"`
	Tier    string   `json:"tier,omitempty"`
	Efforts []string `json:"efforts,omitempty"`
	InputM  *float64 `json:"input_per_million,omitempty"`
	OutputM *float64 `json:"output_per_million,omitempty"`
	Harness string   `json:"harness,omitempty"`
}

type sessionResolver struct {
	harness  string
	session  SessionList
	fallback Resolver
	models   map[string]Model
	efforts  map[string][]string
	rejected map[string]bool
}

// ResolveSessionModels overlays exact session metadata on the optional catalog.
// Ordered operator/hook/discovery lists can rank otherwise unknown IDs; native
// availability order never supplies capability. IDs and prices are not guessed
// from model names. Unknown single-ID capability stays unknown.
func ResolveSessionModels(harness string, session SessionList, fallback Resolver) Resolver {
	if !session.Known {
		return fallback
	}
	r := &sessionResolver{harness: harness, session: session, fallback: fallback, models: map[string]Model{}, efforts: map[string][]string{}, rejected: map[string]bool{}}
	counts := map[string]int{}
	for _, descriptor := range session.Metadata {
		counts[descriptor.ID]++
	}
	for _, descriptor := range session.Metadata {
		if !session.Contains(descriptor.ID) {
			continue
		}
		if counts[descriptor.ID] != 1 || (descriptor.Harness != "" && descriptor.Harness != harness) || !validSessionPrice(descriptor.InputM) || !validSessionPrice(descriptor.OutputM) || !validSessionEfforts(descriptor.Efforts) {
			r.rejected[descriptor.ID] = true
			continue
		}
		tier, hasTier := sessionTier(descriptor.Tier)
		if descriptor.Tier != "" && !hasTier {
			r.rejected[descriptor.ID] = true
			continue
		}
		m, known := Model{}, false
		if fallback != nil {
			m, known = fallback.LookupByID(harness, descriptor.ID)
			if known && m.ID != descriptor.ID {
				// Family matching can supply a capability fallback, but the
				// session did not report that older generation's price or alias.
				m.InputM, m.OutputM, m.CacheReadM, m.CacheWriteM = 0, 0, 0, 0
				m.Native = descriptor.ID
			}
		}
		if hasTier {
			m.Tier, known = tier, true
		}
		if known {
			m.ID, m.Harness = descriptor.ID, harness
			if m.Native == "" {
				m.Native = descriptor.ID
			}
			if descriptor.InputM != nil {
				m.InputM = *descriptor.InputM
			}
			if descriptor.OutputM != nil {
				m.OutputM = *descriptor.OutputM
			}
			r.models[m.ID] = m
		}
		if len(descriptor.Efforts) > 0 {
			r.efforts[descriptor.ID] = append([]string(nil), descriptor.Efforts...)
		}
	}
	// The declared capability order is a fallback only, never native provider
	// inventory order. Keep explicit metadata and catalog facts where available.
	ids := make([]string, 0, len(session.IDs))
	seen := map[string]bool{}
	for _, id := range session.IDs {
		if id != "" && id != inheritID && !seen[id] {
			ids = append(ids, id)
			seen[id] = true
		}
	}
	if !session.NativeAvailability && !session.Unordered && len(ids) >= 2 {
		for index, id := range ids {
			if r.rejected[id] {
				continue
			}
			if _, ok := r.models[id]; ok {
				continue
			}
			if fallback != nil {
				if _, ok := fallback.LookupByID(harness, id); ok {
					continue
				}
			}
			tier := TierMid
			if index == 0 {
				tier = TierSmall
			} else if index == len(ids)-1 {
				tier = TierFrontier
			}
			r.models[id] = Model{ID: id, Native: id, Harness: harness, Tier: tier}
		}
	}
	return r
}

func sessionTier(value string) (Tier, bool) {
	switch value {
	case "small":
		return TierSmall, true
	case "mid":
		return TierMid, true
	case "frontier":
		return TierFrontier, true
	}
	return TierSmall, false
}

func validSessionPrice(value *float64) bool {
	return value == nil || (!math.IsNaN(*value) && !math.IsInf(*value, 0) && *value >= 0)
}

var sessionEffortToken = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,31}$`)

func validSessionEfforts(efforts []string) bool {
	for _, effort := range efforts {
		if !sessionEffortToken.MatchString(effort) {
			return false
		}
	}
	return true
}

// RouteForSession resolves model capabilities before classification and guards.
func RouteForSession(prompt, harness, currentID string, res Resolver, session SessionList) Decision {
	return Route(prompt, harness, currentID, ResolveSessionModels(harness, session, res))
}

func (r *sessionResolver) LookupByID(harness, id string) (Model, bool) {
	if harness == r.harness {
		if r.rejected[id] {
			return Model{}, false
		}
		if m, ok := r.models[id]; ok {
			return m, true
		}
	}
	if r.fallback != nil {
		model, known := r.fallback.LookupByID(harness, id)
		if known && harness == r.harness && r.session.Contains(id) && model.ID != id {
			exact, isExactResolver := r.fallback.(ExactIDResolver)
			if !isExactResolver || !exact.IsExactID(harness, id) {
				model.ID, model.Native = id, id
				model.InputM, model.OutputM, model.CacheReadM, model.CacheWriteM = 0, 0, 0, 0
			}
		}
		return model, known
	}
	return Model{}, false
}

func (r *sessionResolver) ModelFor(harness string, tier Tier) Model {
	// Keep the existing foreign-model guard observable if a supplied resolver
	// violates harness ownership; session evidence must not hide that defect.
	if r.fallback != nil {
		candidate := r.fallback.ModelFor(harness, tier)
		if candidate.ID != "" && candidate.Harness != "" && candidate.Harness != harness {
			return candidate
		}
	}
	if harness == r.harness {
		var best Model
		found := false
		for _, id := range r.session.IDs {
			if id == inheritID || r.IsExplicitOnly(harness, id) {
				continue
			}
			m, ok := r.LookupByID(harness, id)
			if ok && m.Tier == tier && !found {
				best, found = m, true
			}
		}
		if found {
			return best
		}
	}
	if r.fallback != nil {
		return r.fallback.ModelFor(harness, tier)
	}
	return Model{Harness: harness, Tier: tier}
}

func (r *sessionResolver) SavingsRatio(from, to Model) float64 {
	if from.InputM <= 0 || from.OutputM <= 0 || to.InputM <= 0 || to.OutputM <= 0 {
		return 0
	}
	if r.fallback != nil {
		return r.fallback.SavingsRatio(from, to)
	}
	before, after := from.InputM+from.OutputM, to.InputM+to.OutputM
	if before > 0 && after > 0 && after < before {
		return 1 - after/before
	}
	return 0
}

func (r *sessionResolver) EffortFor(harness, id string, effort Effort) string {
	if _, ok := r.efforts[id]; ok && harness == r.harness {
		return effort.String()
	}
	if r.fallback != nil {
		return r.fallback.EffortFor(harness, id, effort)
	}
	return effort.String()
}

func (r *sessionResolver) AllowedEfforts(harness, id string) []string {
	if harness == r.harness {
		if efforts, ok := r.efforts[id]; ok {
			return append([]string(nil), efforts...)
		}
	}
	if provider, ok := r.fallback.(EffortLister); ok {
		return provider.AllowedEfforts(harness, id)
	}
	return nil
}

func (r *sessionResolver) IsExplicitOnly(harness, id string) bool {
	return r.fallback != nil && r.fallback.IsExplicitOnly(harness, id)
}

func (r *sessionResolver) IsExactID(harness, id string) bool {
	if harness == r.harness && r.session.Contains(id) {
		return true
	}
	if exact, ok := r.fallback.(ExactIDResolver); ok {
		return exact.IsExactID(harness, id)
	}
	if r.fallback != nil {
		m, ok := r.fallback.LookupByID(harness, id)
		return ok && m.ID == id
	}
	return false
}

// SessionModelRejected prevents malformed authoritative metadata from reopening
// the ordered-list fallback after LookupByID deliberately refused that ID.
func (r *sessionResolver) SessionModelRejected(harness, id string) bool {
	return harness == r.harness && r.rejected[id]
}

// FamilyModelFor preserves the optional family/effort capability, constrained
// by the same exact session membership as every other model selection.
func (r *sessionResolver) FamilyModelFor(harness, family string, effort Effort) (Model, bool) {
	provider, ok := r.fallback.(FamilyEffortResolver)
	if !ok {
		return Model{}, false
	}
	model, ok := provider.FamilyModelFor(harness, family, effort)
	if !ok || harness != r.harness || !r.session.Contains(model.ID) || r.rejected[model.ID] {
		return Model{}, false
	}
	return r.LookupByID(harness, model.ID)
}
