// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package catalog

// SessionCandidates implements core.SessionCandidateResolver: the model names
// the catalog declares a harness always accepts, least to most capable. Nil
// when the harness declares none. The caller resolves and filters each name.
func (c *Catalog) SessionCandidates(harness string) []string {
	return append([]string(nil), c.sessionCandidates[harness]...)
}

// mergeSessionCandidates lets a user override replace a harness's candidates
// (an empty list turns them off) and keeps every harness it does not name.
func mergeSessionCandidates(base, override map[string][]string) map[string][]string {
	if len(override) == 0 {
		return base
	}
	out := make(map[string][]string, len(base)+len(override))
	for h, names := range base {
		out[h] = names
	}
	for h, names := range override {
		out[h] = names
	}
	return out
}
