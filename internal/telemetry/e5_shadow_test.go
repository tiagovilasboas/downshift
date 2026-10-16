// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

func TestFromDecision_RecordsE5Shadow(t *testing.T) {
	d := core.Decision{
		Harness: "cursor",
		Tier:    core.TierSmall,
		E5Shadow: core.E5Shadow{
			ModelID:      "e5-prototypes-sha256:abc",
			NearestLabel: "COMPLEX",
			Similarity:   0.81234,
			WouldLabel:   "COMPLEX",
			WouldLift:    true,
			RegexLabel:   "TRIVIAL",
			LatencyMs:    12,
		},
	}
	ev := telemetry.FromDecision(d, "0123456789abcdef", "test")
	if ev.E5Shadow == nil {
		t.Fatal("expected e5_shadow record")
	}
	if ev.E5Shadow.ModelID != d.E5Shadow.ModelID || !ev.E5Shadow.WouldLift {
		t.Fatalf("record = %+v", ev.E5Shadow)
	}
	raw, _ := json.Marshal(ev)
	if strings.Contains(string(raw), "prompt") {
		t.Fatalf("telemetry must not contain prompt keys: %s", raw)
	}
}

func TestFromDecision_OmitsE5WhenEmpty(t *testing.T) {
	ev := telemetry.FromDecision(core.Decision{Harness: "cursor"}, "0123456789abcdef", "test")
	if ev.E5Shadow != nil {
		t.Fatalf("unexpected e5_shadow: %+v", ev.E5Shadow)
	}
	b, _ := json.Marshal(ev)
	if strings.Contains(string(b), "e5_shadow") {
		t.Fatalf("json should omit e5_shadow: %s", b)
	}
}
