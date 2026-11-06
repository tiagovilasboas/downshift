// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package fromcore is the edge between the router and the event log: it
// translates core values (Decision, Resolver, harness capabilities) into the
// plain values telemetry owns. telemetry itself never imports core.
package fromcore

import (
	"time"

	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// FromDecision builds a telemetry.Event from a core.Decision and hook runtime
// context.
// It assumes a rewrite was emitted; callers set OutcomeAllow when the adapter
// returned the spawn unchanged.
// Self-correction fields are recorded only for held decisions, keeping clean
// events compact and old log readers unaffected.
func FromDecision(d core.Decision, correlationID, binaryVersion string) telemetry.Event {
	ev := telemetry.Event{
		CorrelationID:   correlationID,
		Timestamp:       time.Now().UTC().Format(time.RFC3339Nano),
		Source:          "hook",
		Agent:           "subagent",
		Harness:         d.Harness,
		Complexity:      d.Complexity.String(),
		FromModel:       telemetry.ModelOrUnknown(d.Baseline.CurrentModel.ID),
		ToModel:         telemetry.ModelOrUnknown(d.Model.ID),
		RequestedEffort: telemetry.EffortUnobserved,
		FinalEffort:     telemetry.EffortOrUnknown(d.Effort.String()),
		Verdict:         d.Verdict.String(),
		Tier:            d.Tier.String(),
		PolicyVersion:   telemetry.PolicyVersion,
		BinaryVersion:   binaryVersion,
		Outcome:         telemetry.OutcomeRewriteEmitted,
		QuotaStatus:     d.Session.QuotaStatus,
		QuotaSource:     d.Session.QuotaSource,
		CreditHeld:      d.Session.CreditHeld,
		PolicyFloor:     d.Guards.PolicyFloor,
		RoutingPolicy:   telemetry.RoutingPolicy,
		ReasonCodes:     ReasonCodes(d),
	}
	if d.Note.Verdict != "" {
		n := telemetry.RoutingNote{
			Verdict: d.Note.Verdict, Reason: d.Note.Reason, RuleID: d.Note.RuleID,
			Profile: d.Note.Profile, TierFrom: d.Note.TierFrom, TierTo: d.Note.TierTo,
		}.Clean()
		if n.Verdict != "" {
			ev.RoutingNote = &n
		}
	}
	if d.Shadow.GatewayWould != "" {
		ev.GatewayWould = telemetry.ModelOrUnknown(d.Shadow.GatewayWould)
		ev.GatewayWouldTier = d.Tier.String()
	}
	if len(d.Corrections) > 0 {
		ev.SafeVerdict = d.SafeVerdict.String()
		ev.Corrections = d.Corrections
	}
	// Both naive-Bayes opinions were removed: Downshift never raises a tier on
	// its own, and no classifier second opinion lowers one. A live decision
	// carries neither, so none is emitted. The telemetry.Event.NBUpshift and
	// NBDownshift fields stay so old events that recorded one still decode
	// (see the report/stats reader).
	return ev
}
