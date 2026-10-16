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

func TestFromDecision_E5ShadowAllowlistedKeysAndClosedError(t *testing.T) {
	const canary = "SECRET_PROMPT_CANARY_e5_tel"
	d := core.Decision{
		Harness: "cursor",
		Tier:    core.TierSmall,
		E5Shadow: core.E5Shadow{
			ModelID:      "e5-prototypes-sha256:deadbeef",
			NearestLabel: "MEDIUM",
			ErrorCode:    "embed_error",
			LatencyMs:    1000,
		},
	}
	ev := telemetry.FromDecision(d, "0123456789abcdef", "test")
	if ev.E5Shadow == nil || ev.E5Shadow.ErrorCode != "embed_error" {
		t.Fatalf("record = %+v", ev.E5Shadow)
	}
	raw, err := json.Marshal(ev)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), canary) {
		t.Fatalf("telemetry leaked canary: %s", raw)
	}
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(raw, &envelope); err != nil {
		t.Fatal(err)
	}
	var rec map[string]any
	if err := json.Unmarshal(envelope["e5_shadow"], &rec); err != nil {
		t.Fatal(err)
	}
	allowed := map[string]struct{}{
		"model_id": {}, "nearest_label": {}, "similarity": {},
		"would_label": {}, "would_lift": {}, "regex_label": {},
		"regex_confident": {}, "latency_ms": {}, "error_code": {},
	}
	for k := range rec {
		if _, ok := allowed[k]; !ok {
			t.Errorf("unexpected e5_shadow key %q", k)
		}
		if strings.Contains(strings.ToLower(k), "prompt") || strings.Contains(k, "path") || strings.Contains(k, "stderr") {
			t.Errorf("forbidden key %q", k)
		}
	}
}
