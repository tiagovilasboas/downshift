// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package fromcore is the edge between the router and the event log: it
// translates core values (Decision, Resolver, harness capabilities) into the
// plain values telemetry owns. telemetry itself never imports core.
package fromcore

import (
	"math"
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
	if d.Shadow.NBUpshift.Would {
		ev.NBUpshift = &telemetry.NBUpshiftRecord{
			FromTier: d.Shadow.NBUpshift.From.String(),
			ToTier:   d.Shadow.NBUpshift.To.String(),
			Margin:   math.Round(d.Shadow.NBUpshift.Margin*1e4) / 1e4,
			Applied:  d.Shadow.NBUpshift.Applied,
		}
	}
	if d.Shadow.NBDownshift.Would {
		// The opinion only exists for the Medium default, so it is mid -> small.
		ev.NBDownshift = &telemetry.NBUpshiftRecord{
			FromTier: core.TierMid.String(),
			ToTier:   core.TierSmall.String(),
			Margin:   math.Round(d.Shadow.NBDownshift.Margin*1e4) / 1e4,
			Applied:  d.Shadow.NBDownshift.Applied,
		}
	}
	return ev
}
