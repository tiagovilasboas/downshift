// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core

// isBaselineModel reports whether a session id names the model the spawn
// already runs. A session list carries the id the harness reported, which may
// be an alias such as "sonnet", while the baseline holds the catalog id, so
// string equality alone would plan a rewrite that writes the same model back:
// a no-op the harness runs unchanged but telemetry would count as applied.
// Identity is the catalog's, so an alias and its id are one model.
func isBaselineModel(d Decision, id string, res Resolver) bool {
	current := d.Baseline.CurrentModel.ID
	if id == current {
		return true
	}
	if res == nil || current == "" {
		return false
	}
	m, ok := res.LookupByID(d.Harness, id)
	return ok && m.ID == current
}
