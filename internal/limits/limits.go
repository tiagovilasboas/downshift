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
)

// Standard status values.
const (
	StatusFull    = "full"    // Fully supported
	StatusPartial = "partial" // Works in some conditions
	StatusNone    = "none"    // Not supported
	StatusManual  = "manual"  // Requires manual invocation
)

// Limitation describes a specific capability limitation of a harness.
type Limitation struct {
	// Feature is the capability name (e.g., "model_rewrite", "token_counts").
	Feature string
	// Status is the support level: "full", "partial", "none", "manual".
	Status string
	// Reason explains why the limitation exists.
	Reason string
}

// HarnessLimits holds all known limitations for a harness.
type HarnessLimits struct {
	HarnessID   string
	Limitations []Limitation
}

// registry holds limitations for all known harnesses. Statuses follow the
// harness matrix (docs/reference/harness-matrix.md): only a Completed harness
// claims full support, and only Completed records usage. parity_test.go holds
// this table to the matrix rows.
var registry = map[string]HarnessLimits{
	"kirocrew": {
		HarnessID: "kirocrew",
		Limitations: []Limitation{
			{
				Feature: "model_rewrite",
				Status:  "none",
				Reason:  "preToolUse hook is allow/block only; no updated_input channel",
			},
			{
				Feature: "token_counts",
				Status:  "none",
				Reason:  "ACP provider does not expose inputTokens/outputTokens (F7); nothing recorded",
			},
			{
				Feature: "compaction",
				Status:  "manual",
				Reason:  "no rewrite channel; agent must run 'downshift compact --' itself",
			},
			{
				Feature: "mid_session_routing",
				Status:  "none",
				Reason:  "closed harness; model fixed at session start",
			},
			{
				Feature: "savings_estimation",
				Status:  "partial",
				Reason:  "uses context_used as proxy; not actual token counts",
			},
		},
	},
	"claude-code": {
		HarnessID: "claude-code",
		Limitations: []Limitation{
			{
				Feature: "model_rewrite",
				Status:  "full",
				Reason:  "rewrite observed honored on a real child (2026-10-06)",
			},
			{
				Feature: "token_counts",
				Status:  "full",
				Reason:  "usage read from the child transcript at SubagentStop",
			},
			{
				Feature: "compaction",
				Status:  "full",
				Reason:  "automatic via preToolUse command rewrite",
			},
			{
				Feature: "mid_session_routing",
				Status:  "full",
				Reason:  "subagent model can be changed per spawn",
			},
			{
				Feature: "savings_estimation",
				Status:  "full",
				Reason:  "based on actual token usage from the child transcript",
			},
		},
	},
	"cursor": {
		HarnessID: "cursor",
		Limitations: []Limitation{
			{
				Feature: "model_rewrite",
				Status:  "partial",
				Reason:  "rewrite emitted; honored unconfirmed, discarded on Free and legacy Pro",
			},
			{
				Feature: "token_counts",
				Status:  "none",
				Reason:  "no usage field is read at subagentStop",
			},
			{
				Feature: "compaction",
				Status:  "partial",
				Reason:  "shell rewrite wired but unverified live; otherwise run 'downshift compact --' by hand",
			},
			{
				Feature: "mid_session_routing",
				Status:  "partial",
				Reason:  "honored unconfirmed: the child executor model is absent from the transcript",
			},
			{
				Feature: "savings_estimation",
				Status:  "partial",
				Reason:  "no usage source; list-price estimate only",
			},
		},
	},
	"codex": {
		HarnessID: "codex",
		Limitations: []Limitation{
			{
				Feature: "model_rewrite",
				Status:  "partial",
				Reason:  "rewrite emitted on a real spawn; honored inferred from the next spawn, not observed",
			},
			{
				Feature: "token_counts",
				Status:  "none",
				Reason:  "no usage source; child end parser unverified",
			},
			{
				Feature: "compaction",
				Status:  "partial",
				Reason:  "Bash PreToolUse rewrite wired, unverified live",
			},
			{
				Feature: "mid_session_routing",
				Status:  "partial",
				Reason:  "same evidence as model_rewrite: honored inferred, not observed",
			},
			{
				Feature: "savings_estimation",
				Status:  "partial",
				Reason:  "no usage source; list-price estimate only",
			},
		},
	},
	"antigravity": {
		HarnessID: "antigravity",
		Limitations: []Limitation{
			{
				Feature: "model_rewrite",
				Status:  "partial",
				Reason:  "one observed child on desktop 2.21.1 (requested pro, executed flash_lite)",
			},
			{
				Feature: "token_counts",
				Status:  "none",
				Reason:  "no child end recorded; postToolUse result fields are undocumented",
			},
			{
				Feature: "compaction",
				Status:  "manual",
				Reason:  "no shell rewrite wired; agent must run 'downshift compact --' itself",
			},
			{
				Feature: "mid_session_routing",
				Status:  "partial",
				Reason:  "one observed child on one build; quota and alias mapping unresolved",
			},
			{
				Feature: "savings_estimation",
				Status:  "partial",
				Reason:  "no usage source; list-price estimate only",
			},
		},
	},
	"opencode": {
		HarnessID: "opencode",
		Limitations: []Limitation{
			{
				Feature: "model_rewrite",
				Status:  "partial",
				Reason:  "route wire v1 and conformance green; forwarding plugin in development, no live spawn observed",
			},
			{
				Feature: "token_counts",
				Status:  "none",
				Reason:  "usage seam not proven; no child end recorded",
			},
			{
				Feature: "compaction",
				Status:  "manual",
				Reason:  "shell rewrite not wired; agent must run 'downshift compact --' itself",
			},
			{
				Feature: "mid_session_routing",
				Status:  "partial",
				Reason:  "route answer computed; forwarding plugin in development, no live spawn observed",
			},
			{
				Feature: "savings_estimation",
				Status:  "partial",
				Reason:  "no usage seam; list-price estimate only",
			},
		},
	},
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
