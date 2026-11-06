// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/accounting"
	"github.com/tiagovilasboas/downshift/internal/execwrap"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

func famEvent(session, family string, raw, emitted int64) execwrap.Event {
	return execwrap.Event{
		TS: "2026-11-01T10:00:00Z", Family: execwrap.Family(family), RawBytes: raw, EmittedBytes: emitted,
		Compacted: true, Session: session, Harness: "claude-code",
	}
}

func TestPanelCompactorFamilies(t *testing.T) {
	events := []execwrap.Event{
		famEvent("mine", "gotest", 4000, 400),
		famEvent("mine", "gotest", 4000, 400),
		famEvent("mine", "git", 4000, 400),
		famEvent("other", "pytest", 40000, 400),
	}
	out := panelWithReport(telemetry.PanelView{Session: "mine"}, "mine", nil, events)
	if out.Compactor == nil {
		t.Fatal("compactor block missing")
	}
	want := []compactorFamilyPanel{
		{Family: "gotest", Commands: 2, Compacted: 2, RawTokens: 2000, AvoidedTokens: 1800},
		{Family: "git", Commands: 1, Compacted: 1, RawTokens: 1000, AvoidedTokens: 900},
	}
	got := out.Compactor.Families
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("families = %+v, want %+v", got, want)
	}
}

func TestPanelCompactorFamiliesNeverNull(t *testing.T) {
	if got := sessionFamilies(nil, accounting.Scope{Kind: accounting.ScopeSession, SessionHash: "none"}); got == nil {
		t.Fatal("families is nil, want an empty slice")
	}
	events := []execwrap.Event{famEvent("mine", "git", 100, 10)}
	raw, err := json.Marshal(panelWithReport(telemetry.PanelView{}, "mine", nil, events))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), `"families":null`) || !strings.Contains(string(raw), `"families":[`) {
		t.Fatalf("families not an array: %s", raw)
	}
}
