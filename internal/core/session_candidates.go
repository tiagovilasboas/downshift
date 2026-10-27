// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core

// SessionCandidateResolver is an optional Resolver extension. It returns the
// model names a harness always accepts in a spawn, ordered least to most
// capable, for a harness whose payload carries no session list. The names are
// catalog data (session_candidates), never Go source.
type SessionCandidateResolver interface {
	SessionCandidates(harness string) []string
}

// CatalogSessionFromCurrent builds the last-resort session list for a harness
// that declares SessionFromCatalog and whose real list could not be learned
// (no payload list, native export, discovery cache or operator file).
//
// The current model is proof of access. The list keeps, per tier strictly
// below the current model's tier, the first catalog candidate that resolves
// to an automatic model, and ends at the current model, so an upshift can
// never go past what the session already runs. An explicit_only candidate is
// never added. An unknown or empty current model is an unknown session, which
// never rewrites.
func CatalogSessionFromCurrent(harness, currentID string, res Resolver) SessionList {
	cands, ok := res.(SessionCandidateResolver)
	if harness == "" || currentID == "" || !ok {
		return UnknownSession()
	}
	cur, ok := res.LookupByID(harness, currentID)
	if !ok || cur.Tier < TierSmall || cur.Tier > TierFrontier {
		return UnknownSession()
	}
	ids := make([]string, 0, 3)
	tiers := map[Tier]bool{}
	for _, name := range cands.SessionCandidates(harness) {
		m, ok := res.LookupByID(harness, name)
		if !ok || m.ID == "" || m.ID == cur.ID || m.Tier >= cur.Tier || m.Tier < TierSmall || tiers[m.Tier] {
			continue
		}
		if res.IsExplicitOnly(harness, m.ID) {
			continue
		}
		tiers[m.Tier] = true
		ids = append(ids, m.ID)
	}
	s := KnownSession(append(ids, currentID))
	s.FromCatalog = true
	return s
}
