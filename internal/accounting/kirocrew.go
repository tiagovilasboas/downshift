// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package accounting

import (
	"os"

	"github.com/tiagovilasboas/downshift/internal/discovery"
)

// GetKiroCrewMetrics returns session metrics for the current KiroCrew session.
// Returns nil if not running in KiroCrew or metrics cannot be determined.
func GetKiroCrewMetrics() *KiroCrewSessionMetrics {
	sessionID := os.Getenv("KIRO_SESSION_ID")
	if sessionID == "" {
		return nil
	}

	metrics, err := discovery.InferKiroCrewMetrics(sessionID)
	if err != nil {
		return nil
	}

	return &KiroCrewSessionMetrics{
		SessionID:          sessionID,
		Model:              metrics.Model,
		ContextUsed:        metrics.ContextUsed,
		ContextWindow:      metrics.ContextWindow,
		ContextUtilization: metrics.ContextUtilization(),
	}
}

// EstimateContextSavings estimates how many tokens were saved by compaction
// relative to context usage. This is a directional estimate:
//   - SavedBytes / 4 gives estimated tokens saved by compactor
//   - ContextUsed gives how much context the session is consuming
//
// The ratio shows what percentage of context would have been consumed
// without compaction. This is useful for understanding compactor impact
// in the context of the session's context budget.
func EstimateContextSavings(compactorSaved int64, contextUsed int64) float64 {
	if contextUsed <= 0 {
		return 0
	}
	estimatedTokensSaved := compactorSaved / 4 // same ratio as BytesPerToken
	if estimatedTokensSaved <= 0 {
		return 0
	}
	// What % of current context would output have been without compaction?
	hypotheticalContext := contextUsed + estimatedTokensSaved
	return float64(estimatedTokensSaved) / float64(hypotheticalContext) * 100
}
