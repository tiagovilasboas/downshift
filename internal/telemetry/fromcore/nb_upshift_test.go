// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package fromcore

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/core"

	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// The NB upshift second opinion was removed: a live decision never carries
// one, so FromDecision never emits nb_upshift on a fresh event.
func TestFromDecision_NeverEmitsNBUpshift(t *testing.T) {
	ev := FromDecision(core.Decision{Tier: core.TierSmall}, "0123456789abcdef", "test")
	b, _ := json.Marshal(ev)
	if ev.NBUpshift != nil || strings.Contains(string(b), "nb_upshift") {
		t.Fatalf("unexpected nb_upshift: %s", b)
	}
}

// A shadow NB TRIVIAL downshift is recorded as mid -> small, not applied.
func TestFromDecision_RecordsNBShadowDownshift(t *testing.T) {
	d := core.Decision{Tier: core.TierMid, Shadow: core.Shadow{NBDownshift: core.NBDownshift{Would: true, Margin: 0.31234}}}
	ev := FromDecision(d, "0123456789abcdef", "test")
	want := telemetry.NBUpshiftRecord{FromTier: "mid", ToTier: "small", Margin: 0.3123, Applied: false}
	if ev.Tier != "mid" || ev.NBDownshift == nil || *ev.NBDownshift != want {
		t.Fatalf("event tier=%s nb_downshift=%+v, want %+v", ev.Tier, ev.NBDownshift, want)
	}
	clean := FromDecision(core.Decision{Tier: core.TierMid}, "0123456789abcdef", "test")
	if clean.NBDownshift != nil {
		t.Fatalf("unexpected nb_downshift on a clean decision: %+v", clean.NBDownshift)
	}
}
