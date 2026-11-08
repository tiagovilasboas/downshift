// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package fromcore

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/core"

	"github.com/tiagovilasboas/downshift-ai/internal/telemetry"
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

// The NB TRIVIAL downshift opinion was removed too: FromDecision never emits
// nb_downshift or its reason code on a fresh event.
func TestFromDecision_NeverEmitsNBDownshift(t *testing.T) {
	ev := FromDecision(core.Decision{Tier: core.TierMid, NoSignal: true}, "0123456789abcdef", "test")
	b, _ := json.Marshal(ev)
	if ev.NBDownshift != nil || strings.Contains(string(b), "nb_downshift") ||
		strings.Contains(string(b), telemetry.ReasonNBDownshiftApplied) {
		t.Fatalf("unexpected naive-Bayes downshift: %s", b)
	}
}

// An old event that recorded an NB downshift still decodes, so a log written
// before the opinion was removed reads unchanged.
func TestOldNBDownshiftEventStillDecodes(t *testing.T) {
	raw := `{"harness":"claude-code","tier":"mid","nb_downshift":{"from_tier":"mid","to_tier":"small","margin":0.3123,"applied":true},"reason_codes":["NB_DOWNSHIFT_APPLIED"]}`
	var ev telemetry.Event
	if err := json.Unmarshal([]byte(raw), &ev); err != nil {
		t.Fatal(err)
	}
	want := telemetry.NBUpshiftRecord{FromTier: "mid", ToTier: "small", Margin: 0.3123, Applied: true}
	if ev.NBDownshift == nil || *ev.NBDownshift != want {
		t.Fatalf("nb_downshift = %+v, want %+v", ev.NBDownshift, want)
	}
}
