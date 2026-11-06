// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package hookutil

import "github.com/tiagovilasboas/downshift/internal/core"

// EffortRank orders the cross-harness effort vocabulary. Unknown values rank -1.
func EffortRank(v string) int {
	for i, e := range []string{"none", "minimal", "low", "medium", "high", "xhigh", "max", "ultra"} {
		if v == e {
			return i
		}
	}
	return -1
}

// EffortAllowed reports whether a hook may write effort value over the requested
// one. A confident, unheld decision may set any effort. A decision the
// guardrails held writes no effort. An unconfident, unheld decision writes
// effort only alongside a model rewrite, and never lowers it.
func EffortAllowed(d core.Decision, requested, value string, modelRewritten bool) bool {
	if d.Confident && len(d.Corrections) == 0 {
		return true
	}
	if d.Checked && d.SafeVerdict != d.Verdict {
		return false
	}
	if !modelRewritten {
		return false
	}
	if EffortRank(requested) < 0 {
		return true
	}
	return EffortRank(value) >= EffortRank(requested)
}
