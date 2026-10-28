// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package loop

import (
	"testing"

	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

func acc(session, state string) telemetry.Event {
	return telemetry.Event{Outcome: telemetry.OutcomeAcceptance, SessionID: session, Acceptance: state}
}

func TestBuildReport(t *testing.T) {
	events := []telemetry.Event{
		acc("s2", "used"),
		acc("s1", "used"),
		acc("s1", "discarded"),
		acc("s1", "unknown"),
		{Outcome: "usage", SessionID: "s1"},
	}
	r := BuildReport(events, "")
	if r.Used != 2 || r.Discarded != 1 || r.Unknown != 1 {
		t.Fatalf("counts = %+v", r)
	}
	if r.AcceptanceRate == nil || *r.AcceptanceRate < 0.66 || *r.AcceptanceRate > 0.67 {
		t.Errorf("rate = %v, want 2/3", r.AcceptanceRate)
	}
	if len(r.Sessions) != 2 || r.Sessions[0].SessionID != "s1" {
		t.Errorf("sessions = %+v, want s1 then s2", r.Sessions)
	}
}

func TestBuildReportSessionFilter(t *testing.T) {
	r := BuildReport([]telemetry.Event{acc("s1", "used"), acc("s2", "discarded")}, "s2")
	if r.Used != 0 || r.Discarded != 1 || len(r.Sessions) != 1 {
		t.Errorf("filtered = %+v", r)
	}
}

func TestBuildReportNoDecisionHasNoRate(t *testing.T) {
	r := BuildReport([]telemetry.Event{acc("s1", "unknown")}, "")
	if r.AcceptanceRate != nil {
		t.Errorf("rate = %v, want nil when nothing was decided", *r.AcceptanceRate)
	}
}
