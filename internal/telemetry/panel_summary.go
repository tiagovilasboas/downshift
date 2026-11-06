// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"fmt"
	"strings"
)

// TokensShort is a token count in the CLI's format ("58k", "1.2M"), "" when
// there is nothing to show.
func TokensShort(n int64) string {
	if n <= 0 {
		return ""
	}
	return formatTokens(n)
}

// PanelSummary is the band's one line: what Downshift changed and held in this
// session, in plain words, e.g. "Downshift · 2 model · 1 effort · 1 held". A
// zero count is left out; with nothing changed or held the line is "" (the band
// then says nothing about Downshift).
func PanelSummary(v PanelView) string {
	var parts []string
	if v.ModelSwitched > 0 {
		parts = append(parts, fmt.Sprintf("%d model", v.ModelSwitched))
	}
	if v.EffortChanged > 0 {
		parts = append(parts, fmt.Sprintf("%d effort", v.EffortChanged))
	}
	if v.Held > 0 {
		parts = append(parts, fmt.Sprintf("%d held", v.Held))
	}
	if len(parts) == 0 {
		return ""
	}
	return "Downshift · " + strings.Join(parts, " · ")
}
