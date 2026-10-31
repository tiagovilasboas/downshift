// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package limits provides transparent reporting of harness limitations.
// Each harness has different capabilities: some support model rewriting,
// some expose token counts, some allow automatic compaction. This package
// centralizes that knowledge so commands like doctor, stats, and help can
// be frank about what works and what doesn't.
package limits

// Standard feature names. All harnesses are evaluated against these.
const (
	FeatureModelRewrite      = "model_rewrite"       // Can the hook change the subagent model?
	FeatureTokenCounts       = "token_counts"        // Does the harness expose actual token usage?
	FeatureCompaction        = "compaction"          // Is output compaction automatic or manual?
	FeatureMidSessionRouting = "mid_session_routing" // Can model be changed during session?
	FeatureSavingsEstimation = "savings_estimation"  // Based on real tokens or proxy?
	FeatureMCP               = "mcp"                 // Do the read-only MCP tools answer in this surface?
	FeatureEffort            = "effort"              // Can a hook set the subagent's reasoning effort?
)

// Standard status values.
const (
	StatusFull    = "full"    // Fully supported
	StatusPartial = "partial" // Works in some conditions
	StatusNone    = "none"    // Not supported
	StatusManual  = "manual"  // Requires manual invocation
	// StatusUnverified marks a feature nobody has observed working or failing.
	// It is not a support level: it says the evidence is missing.
	StatusUnverified = "unverified"
)

// Limitation describes a specific capability limitation of a harness.
type Limitation struct {
	// Feature is the capability name (e.g., "model_rewrite", "token_counts").
	Feature string
	// Status is the support level: "full", "partial", "none", "manual",
	// or "unverified" when no evidence exists either way.
	Status string
	// Reason explains why the limitation exists.
	Reason string
}

// HarnessLimits holds all known limitations for a harness.
type HarnessLimits struct {
	HarnessID   string
	Limitations []Limitation
}

// ForHarness returns the limitations for a specific harness.
// Returns nil if the harness is unknown.
func ForHarness(harnessID string) *HarnessLimits {
	if lims, ok := registry[harnessID]; ok {
		return &lims
	}
	return nil
}

// All returns limitations for all known harnesses.
func All() map[string]HarnessLimits {
	return registry
}

// HasFullSupport returns true if the harness has full support for a feature.
func (h *HarnessLimits) HasFullSupport(feature string) bool {
	if h == nil {
		return false
	}
	for _, lim := range h.Limitations {
		if lim.Feature == feature {
			return lim.Status == "full"
		}
	}
	return false
}

// GetLimitation returns the limitation for a specific feature.
func (h *HarnessLimits) GetLimitation(feature string) *Limitation {
	if h == nil {
		return nil
	}
	for _, lim := range h.Limitations {
		if lim.Feature == feature {
			return &lim
		}
	}
	return nil
}

// UnsupportedFeatures returns all features that are not fully supported.
func (h *HarnessLimits) UnsupportedFeatures() []Limitation {
	if h == nil {
		return nil
	}
	var result []Limitation
	for _, lim := range h.Limitations {
		if lim.Status != "full" {
			result = append(result, lim)
		}
	}
	return result
}

// SummaryLine returns a one-line summary of limitations for display.
func (h *HarnessLimits) SummaryLine() string {
	if h == nil {
		return "unknown harness"
	}
	unsupported := h.UnsupportedFeatures()
	if len(unsupported) == 0 {
		return "full support"
	}
	features := make([]string, len(unsupported))
	for i, lim := range unsupported {
		features[i] = lim.Feature + ":" + lim.Status
	}
	return joinFeatures(features)
}

func joinFeatures(features []string) string {
	if len(features) == 0 {
		return ""
	}
	result := features[0]
	for i := 1; i < len(features); i++ {
		result += ", " + features[i]
	}
	return result
}
