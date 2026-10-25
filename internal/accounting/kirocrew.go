// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package accounting

// NewKiroCrewSessionMetrics creates session metrics from raw values.
// The caller is responsible for obtaining these values from discovery.InferKiroCrewMetrics.
// This separation maintains the layer boundary: accounting does not import discovery.
func NewKiroCrewSessionMetrics(sessionID, model string, contextUsed, contextWindow int64) *KiroCrewSessionMetrics {
	var utilization float64
	if contextWindow > 0 {
		utilization = float64(contextUsed) / float64(contextWindow)
	}
	return &KiroCrewSessionMetrics{
		SessionID:          sessionID,
		Model:              model,
		ContextUsed:        contextUsed,
		ContextWindow:      contextWindow,
		ContextUtilization: utilization,
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
