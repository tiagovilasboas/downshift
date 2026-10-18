// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package quotamodel

import (
	"testing"
	"time"
)

func number(v float64) *float64 { return &v }

func fixture(now time.Time) Snapshot {
	return Snapshot{Version: 1, Harness: "codex", Source: "codex-transcript", ObservedAt: now.Add(-time.Minute), ExpiresAt: now.Add(time.Minute), Windows: []Window{{ID: "shared", Scope: "harness", UsedPercent: number(40), ResetsAt: now.Add(time.Hour)}}}
}

func TestEvidenceStatesAndHarnessIsolation(t *testing.T) {
	now := time.Now().UTC()
	for _, tc := range []struct {
		name           string
		change         func(*Snapshot)
		harness, model string
		want           Status
	}{
		{"shared budget applies without inventing per-model amounts", func(*Snapshot) {}, "codex", "unknown-model", Available},
		{"wrong harness", func(*Snapshot) {}, "cursor", "unknown-model", Unknown},
		{"exhausted", func(s *Snapshot) { s.Windows[0].UsedPercent = number(100) }, "codex", "m", Exhausted},
		{"missing percentage", func(s *Snapshot) { s.Windows[0].UsedPercent = nil }, "codex", "m", Unknown},
		{"future observation", func(s *Snapshot) { s.ObservedAt = now.Add(time.Second) }, "codex", "m", Unknown},
		{"expired snapshot", func(s *Snapshot) { s.ExpiresAt = now }, "codex", "m", Stale},
		{"reset reached", func(s *Snapshot) { s.Windows[0].ResetsAt = now }, "codex", "m", Stale},
		{"missing reset", func(s *Snapshot) { s.Windows[0].ResetsAt = time.Time{} }, "codex", "m", Unknown},
		{"invalid percentage", func(s *Snapshot) { s.Windows[0].UsedPercent = number(101) }, "codex", "m", Unknown},
		{"unbounded TTL", func(s *Snapshot) { s.ExpiresAt = s.ObservedAt.Add(24 * time.Hour) }, "codex", "m", Unknown},
		{"model pool exact", func(s *Snapshot) { s.Windows[0].Scope = "models"; s.Windows[0].ModelIDs = []string{"m"} }, "codex", "m", Available},
		{"model pool cannot leak", func(s *Snapshot) { s.Windows[0].Scope = "models"; s.Windows[0].ModelIDs = []string{"m"} }, "codex", "m-new-version", Unknown},
		{"secondary exhausted vetoes primary", func(s *Snapshot) {
			s.Windows = append(s.Windows, Window{ID: "secondary", Scope: "harness", UsedPercent: number(100), ResetsAt: now.Add(time.Hour)})
		}, "codex", "m", Exhausted},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := fixture(now)
			tc.change(&s)
			if got := s.Evaluate(tc.harness, tc.model, now); got != tc.want {
				t.Fatalf("got %s want %s", got, tc.want)
			}
		})
	}
}

func TestAbandonedBoundary(t *testing.T) {
	now := time.Now().UTC()
	s := Snapshot{Version: 1, Harness: "h", Source: "operator-normalized", ObservedAt: now.Add(-time.Hour - time.Minute), ExpiresAt: now.Add(-time.Hour)}
	if s.Abandoned(now) {
		t.Fatal("exactly one hour after expiry must still hold")
	}
	if !s.Abandoned(now.Add(time.Nanosecond)) {
		t.Fatal("past one hour after expiry must be abandoned")
	}
	if (Snapshot{Version: 1, Harness: "h", Source: "bogus"}).Abandoned(now) {
		t.Fatal("malformed snapshot must never be abandoned")
	}
}

func TestSourceNameKeepsKnownProvenance(t *testing.T) {
	if SourceName("cursor-usage") != "cursor-usage" || SourceName("invented") != "" {
		t.Fatal("source vocabulary changed")
	}
}
