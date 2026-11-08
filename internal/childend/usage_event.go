// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package childend

import (
	"slices"
	"time"

	"github.com/tiagovilasboas/downshift-ai/internal/core"
	"github.com/tiagovilasboas/downshift-ai/internal/hookport"
	"github.com/tiagovilasboas/downshift-ai/internal/telemetry"
)

// buildUsageEvent assembles the NEW cost record. Provenance (harness,
// complexity, models, verdict, tier, session) is copied from the matched
// decision when present; unmatched records use explicit "unknown" markers.
// Token pointers are set only for positive counts so absent usage stays absent
// in JSON (omitempty), and costs are filled only when the spend is real
// (tokens > 0) and the routed model resolved against the catalog.
func buildUsageEvent(harness string, usage telemetry.TokenUsage, payloadModel, sessionHash, binaryVersion string, target *telemetry.Event, r core.Resolver, now time.Time) telemetry.Event {
	ev := telemetry.Event{
		CorrelationID: telemetry.NewCorrelationID(),
		Timestamp:     now.UTC().Format(time.RFC3339Nano),
		Source:        "hook",
		Agent:         "subagent",
		Harness:       harness,
		Outcome:       telemetry.OutcomeUsage,
		PolicyVersion: telemetry.PolicyVersion,
		BinaryVersion: binaryVersion,
		Verdict:       "UNKNOWN",
		FromModel:     "unknown",
		ToModel:       telemetry.ModelOrUnknown(payloadModel),
	}
	if target != nil {
		ev.Harness = target.Harness
		ev.Complexity = target.Complexity
		ev.FromModel = target.FromModel
		ev.ToModel = ranModel(*target)
		ev.Verdict = target.Verdict
		ev.Tier = target.Tier
		ev.SessionID = target.SessionID
		ev.LinkedDecision = decisionKey(*target)
		if payloadModel != "" {
			ev.ToModel = telemetry.ModelOrUnknown(payloadModel)
		}
	}
	if ev.SessionID == "" {
		ev.SessionID = sessionHash
	}
	if usage.InputTokens > 0 {
		v := usage.InputTokens
		ev.InputTokens = &v
	}
	if usage.OutputTokens > 0 {
		v := usage.OutputTokens
		ev.OutputTokens = &v
	}
	if usage.CachedTokens > 0 {
		v := usage.CachedTokens
		ev.CachedTokens = &v
	}
	if usage.CacheWriteTokens > 0 {
		v := usage.CacheWriteTokens
		ev.CacheWriteTokens = &v
	}

	// Set the requested baseline only against a catalog-resolved routed model.
	// The token counts above are already on the record; this only decides
	// whether a baseline can be attributed. No price is read.
	routedID := payloadModel
	if routedID == "" && target != nil {
		// With no observed model and no known requested model, a control
		// spawn has no baseline; attributing the routed one on its rewrite
		// target would keep it in one report arm only.
		if target.FromModel == "" || target.FromModel == "unknown" {
			return ev
		}
		routedID = ranModel(*target)
	}
	if r == nil || routedID == "" || routedID == "unknown" {
		return ev // no resolved model: the tokens are recorded, the baseline is not
	}
	if _, ok := r.LookupByID(harness, routedID); !ok {
		return ev
	}
	if usage == (telemetry.TokenUsage{}) {
		return ev
	}
	// Tokens only: the record carries the token counts and the model ids; the
	// tier comparison reads those. No dollar figure is written.
	// Observed routed spend does not establish the requested baseline. Keep
	// the source absent unless the linked decision names a catalog-resolved model.
	if target != nil && target.FromModel != "" && target.FromModel != "unknown" {
		if _, ok := r.LookupByID(harness, target.FromModel); ok {
			ev.BaselineSource = telemetry.BaselineRequested
			if slices.Contains(target.ReasonCodes, telemetry.ReasonBaselineFromSettings) {
				ev.BaselineSource = telemetry.BaselineInferred
			}
		}
	}
	return ev
}

// gatewayRanModel returns the OpenRouter slug the child ran on, when the harness
// reported one: the launch observation first, then the payload's resolved and
// usage models. "" for a native run.
func gatewayRanModel(resolved *telemetry.Event, ce hookport.ChildEnd) string {
	candidates := []string{ce.ResolvedModel, ce.Model}
	if resolved != nil {
		candidates = append([]string{resolved.ToModel}, candidates...)
	}
	for _, c := range candidates {
		if telemetry.IsGatewayModel(c) {
			return c
		}
	}
	return ""
}

// markGatewayRun names the model an OpenRouter route ran, so the record is
// placed by what ran and not by the requested tier. No price is recorded.
func markGatewayRun(ev *telemetry.Event, slug string) {
	// The slug comes from the harness payload: bound it like the native path.
	ev.ToModel = telemetry.ModelOrUnknown(slug)
}

// addUsage sums two token counts field by field.
func addUsage(a, b telemetry.TokenUsage) telemetry.TokenUsage {
	return telemetry.TokenUsage{
		InputTokens:      a.InputTokens + b.InputTokens,
		OutputTokens:     a.OutputTokens + b.OutputTokens,
		CachedTokens:     a.CachedTokens + b.CachedTokens,
		CacheWriteTokens: a.CacheWriteTokens + b.CacheWriteTokens,
	}
}

// usageGrowth is total minus recorded, field by field, never below zero: what a
// cumulative transcript gained since the last record.
func usageGrowth(total, recorded telemetry.TokenUsage) telemetry.TokenUsage {
	grow := func(t, r int64) int64 { return max(0, t-r) }
	return telemetry.TokenUsage{
		InputTokens:      grow(total.InputTokens, recorded.InputTokens),
		OutputTokens:     grow(total.OutputTokens, recorded.OutputTokens),
		CachedTokens:     grow(total.CachedTokens, recorded.CachedTokens),
		CacheWriteTokens: grow(total.CacheWriteTokens, recorded.CacheWriteTokens),
	}
}
