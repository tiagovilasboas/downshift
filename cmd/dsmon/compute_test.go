// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"testing"
	"time"
)

// dsmon counts savings only for applied rewrites and shows held decisions
// as unchanged.
func TestCompute_CountsOnlyAppliedRewrites(t *testing.T) {
	ts := time.Now().UTC().Format(time.RFC3339Nano)
	all := []event{
		{Timestamp: ts, Harness: "claude-code", Verdict: "DOWNSHIFT", Savings: 0.8, Outcome: "rewrite_emitted", QuotaStatus: "available"},
		{Timestamp: ts, Harness: "claude-code", Verdict: "DOWNSHIFT", Savings: 0.4, Outcome: "allow"},
		{Timestamp: ts, Harness: "codex", Verdict: "DOWNSHIFT", Savings: 0.5, Outcome: "rewrite_emitted", Corrections: []string{"R1_UNCONFIDENT_DOWNSHIFT"}},
		{Timestamp: ts, Harness: "claude-code", Verdict: "DOWNSHIFT", Outcome: "usage"},
	}
	st, recent := compute(all)
	if st.total != 3 || st.down != 1 || st.held != 2 {
		t.Fatalf("total=%d down=%d held=%d, want 3/1/2", st.total, st.down, st.held)
	}
	if st.totalSavingsUnits < 0.79 || st.totalSavingsUnits > 0.81 {
		t.Fatalf("savings units=%.2f, want 0.8", st.totalSavingsUnits)
	}
	if len(recent) != 3 || recent[1].Verdict != "HELD" {
		t.Fatalf("held decision must render as HELD, got %+v", recent)
	}
}
