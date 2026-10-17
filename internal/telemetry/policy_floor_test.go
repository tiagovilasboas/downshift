// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

func TestFromDecisionRecordsPolicyFloor(t *testing.T) {
	if ev := telemetry.FromDecision(core.Decision{Harness: "cursor"}, "id", "v"); ev.PolicyFloor {
		t.Fatal("no floor must not be recorded")
	}
	ev := telemetry.FromDecision(core.Decision{Harness: "cursor", Guards: core.Guards{PolicyFloor: true}}, "id", "v")
	if !ev.PolicyFloor {
		t.Fatal("floor must be recorded")
	}
	raw, _ := json.Marshal(telemetry.FromDecision(core.Decision{Harness: "cursor"}, "id", "v"))
	if strings.Contains(string(raw), "policy_floor") {
		t.Fatal("absent floor must stay out of the JSON so old readers are unaffected")
	}
}

func TestAggregateCountsPolicyFloorAcrossHarnesses(t *testing.T) {
	a := makeEvent("cursor", "TRIVIAL", "OK", 0)
	a.PolicyFloor = true
	b := makeEvent("codex", "SIMPLE", "OK", 0)
	b.PolicyFloor = true
	c := makeEvent("claude-code", "SIMPLE", "OK", 0)
	s := telemetry.Aggregate([]telemetry.Event{a, b, c})
	if s.PolicyFloor != 2 || s.Total != 3 {
		t.Fatalf("PolicyFloor = %d of %d, want 2 of 3", s.PolicyFloor, s.Total)
	}
	var out bytes.Buffer
	telemetry.PrintStats([]telemetry.Event{a, b, c}, telemetry.StatsOptions{Detail: true}, &out)
	if !strings.Contains(out.String(), "Raised by project floor") {
		t.Fatalf("stats must show the floor line, got:\n%s", out.String())
	}
	exp := telemetry.ExportSummary([]telemetry.Event{a, b, c}, 0, time.Now())
	if exp.PolicyFloor != 2 {
		t.Fatalf("export PolicyFloor = %d, want 2", exp.PolicyFloor)
	}
	var none bytes.Buffer
	telemetry.PrintStats([]telemetry.Event{c}, telemetry.StatsOptions{Detail: true}, &none)
	if strings.Contains(none.String(), "project floor") {
		t.Fatal("the line must be hidden when no floor applied")
	}
}
