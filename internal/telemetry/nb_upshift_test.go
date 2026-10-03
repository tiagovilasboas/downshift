// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: BUSL-1.1

package telemetry

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/tiagovilasboas/harness-downshift/internal/core"
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
