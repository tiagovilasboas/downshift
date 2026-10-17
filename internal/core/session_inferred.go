// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core

// InferSessionFromCurrent builds the last-resort session list for a harness
// whose real list could not be learned (no hook list, native export,
// discovery cache or operator file). The current model is proof of access, and
// the catalog tier ladder below it is reachable by downshift. The ladder is
// ordered least to most capable and ends at the current model, so upshift can
// never go past what the session already runs.
//
// It is model-agnostic: tiers come from Resolver.ModelFor, never from names.
// An unknown or empty current model, or a disabled layer, is an unknown
// session, which never rewrites.
func InferSessionFromCurrent(harness, currentID string, res Resolver) SessionList {
	if harness == "" || currentID == "" || res == nil || inferSessionOff() {
		return UnknownSession()
	}
	cur, ok := res.LookupByID(harness, currentID)
	if !ok || cur.Tier < TierSmall || cur.Tier > TierFrontier {
		return UnknownSession()
	}
	ids := make([]string, 0, 3)
	seen := map[string]bool{currentID: true}
	for t := TierSmall; t < cur.Tier; t++ {
		m := res.ModelFor(harness, t)
		if m.ID == "" || m.Tier != t || seen[m.ID] {
			continue
		}
		seen[m.ID] = true
		ids = append(ids, m.ID)
	}
	s := KnownSession(append(ids, currentID))
	s.Inferred = true
	return s
}
