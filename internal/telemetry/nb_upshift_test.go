// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/core"
)

// A shadow NB upshift is recorded on the event without changing its tier.
func TestFromDecision_RecordsNBShadowUpshift(t *testing.T) {
	d := core.Decision{Tier: core.TierMid, NBUpshift: core.NBUpshift{
		Would: true, From: core.TierMid, To: core.TierFrontier, Margin: 0.123456,
	}}
	ev := FromDecision(d, "0123456789abcdef", "test")
	if ev.Tier != "mid" || ev.NBUpshift == nil {
		t.Fatalf("event tier=%s nb=%+v", ev.Tier, ev.NBUpshift)
	}
	want := NBUpshiftRecord{FromTier: "mid", ToTier: "frontier", Margin: 0.1235, Applied: false}
	if *ev.NBUpshift != want {
		t.Fatalf("nb record %+v, want %+v", *ev.NBUpshift, want)
	}
}

// No upshift opinion: the field is omitted so clean events stay compact.
func TestFromDecision_OmitsNBWhenNoUpshift(t *testing.T) {
	ev := FromDecision(core.Decision{Tier: core.TierSmall}, "0123456789abcdef", "test")
	b, _ := json.Marshal(ev)
	if ev.NBUpshift != nil || strings.Contains(string(b), "nb_upshift") {
		t.Fatalf("unexpected nb_upshift: %s", b)
	}
}

// A shadow NB TRIVIAL downshift is recorded as mid -> small, not applied.
func TestFromDecision_RecordsNBShadowDownshift(t *testing.T) {
	d := core.Decision{Tier: core.TierMid, NBDownshift: core.NBDownshift{Would: true, Margin: 0.31234}}
	ev := FromDecision(d, "0123456789abcdef", "test")
	want := NBUpshiftRecord{FromTier: "mid", ToTier: "small", Margin: 0.3123, Applied: false}
	if ev.Tier != "mid" || ev.NBDownshift == nil || *ev.NBDownshift != want {
		t.Fatalf("event tier=%s nb_downshift=%+v, want %+v", ev.Tier, ev.NBDownshift, want)
	}
	clean := FromDecision(core.Decision{Tier: core.TierMid}, "0123456789abcdef", "test")
	if clean.NBDownshift != nil {
		t.Fatalf("unexpected nb_downshift on a clean decision: %+v", clean.NBDownshift)
	}
}
