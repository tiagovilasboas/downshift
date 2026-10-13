// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift/internal/quota"
)

// storeExpiredSnapshot writes a valid cache snapshot that expired expiredAgo
// ago, with a harness window that was not yet reset.
func storeExpiredSnapshot(t *testing.T, harness string, expiredAgo time.Duration) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "quota.json")
	t.Setenv("DOWNSHIFT_QUOTA_FILE", path)
	expires := time.Now().Add(-expiredAgo)
	used := 10.0
	s := quota.Snapshot{Version: 1, Harness: harness, Source: "operator-normalized",
		ObservedAt: expires.Add(-time.Minute), ExpiresAt: expires,
		Windows: []quota.Window{{ID: "w", Scope: "harness", UsedPercent: &used, ResetsAt: time.Now().Add(24 * time.Hour)}}}
	if err := quota.Store(path, s); err != nil {
		t.Fatal(err)
	}
}

// A snapshot that expired less than an hour ago still holds the rewrite; past
// that it is absent and routing behaves as for a user without quota.
func TestStaleQuotaHoldsForAnHourThenBecomesUnknown(t *testing.T) {
	t.Setenv("DOWNSHIFT_QUOTA_MODE", "")
	storeExpiredSnapshot(t, "claude-code", 59*time.Minute)
	s := KnownSession([]string{"candidate"}).WithUsageQuota("claude-code", nil)
	if s.QuotaStatus("claude-code", "candidate") != quota.Stale || CanWriteSessionID("claude-code", "candidate", s, nil) {
		t.Fatalf("snapshot expired 59m ago must hold: status=%s", s.QuotaStatus("claude-code", "candidate"))
	}
	if _, held := StaleQuotaHold("claude-code", time.Now()); !held {
		t.Fatal("StaleQuotaHold missed a holding snapshot")
	}

	storeExpiredSnapshot(t, "claude-code", 61*time.Minute)
	s = KnownSession([]string{"candidate"}).WithUsageQuota("claude-code", nil)
	if s.Usage != nil || s.QuotaRequired || s.QuotaStatus("claude-code", "candidate") != quota.Unknown {
		t.Fatalf("snapshot expired 61m ago must be absent: usage=%v required=%v", s.Usage, s.QuotaRequired)
	}
	if !CanWriteSessionID("claude-code", "candidate", s, nil) {
		t.Fatal("abandoned snapshot must not hold the rewrite")
	}
	if _, held := StaleQuotaHold("claude-code", time.Now()); held {
		t.Fatal("StaleQuotaHold must be false once the snapshot is abandoned")
	}
}

func TestStaleQuotaRequiredModeKeepsHolding(t *testing.T) {
	t.Setenv("DOWNSHIFT_QUOTA_MODE", "required")
	storeExpiredSnapshot(t, "claude-code", 3*time.Hour)
	s := KnownSession([]string{"candidate"}).WithUsageQuota("claude-code", nil)
	if s.QuotaStatus("claude-code", "candidate") != quota.Stale || CanWriteSessionID("claude-code", "candidate", s, nil) {
		t.Fatal("required mode must keep holding on an old snapshot")
	}
	if age, held := StaleQuotaHold("claude-code", time.Now()); !held || age < 3*time.Hour {
		t.Fatalf("required mode: held=%v age=%v", held, age)
	}
}

// Malformed evidence is not abandoned: it keeps closing the gate.
func TestMalformedQuotaCacheStillHolds(t *testing.T) {
	t.Setenv("DOWNSHIFT_QUOTA_MODE", "")
	path := filepath.Join(t.TempDir(), "quota.json")
	t.Setenv("DOWNSHIFT_QUOTA_FILE", path)
	if err := os.WriteFile(path, []byte(`{"version":`), 0600); err != nil {
		t.Fatal(err)
	}
	s := KnownSession([]string{"candidate"}).WithUsageQuota("claude-code", nil)
	if CanWriteSessionID("claude-code", "candidate", s, nil) {
		t.Fatal("malformed cache must hold")
	}
	if _, held := StaleQuotaHold("claude-code", time.Now()); held {
		t.Fatal("malformed cache is not a stale hold")
	}
}

// End to end through the shared planner: the same downshift is held by a
// snapshot 30 minutes past expiry and goes through once it is 2 hours past.
func TestStaleQuotaGatesPlanForSession(t *testing.T) {
	t.Setenv("DOWNSHIFT_QUOTA_MODE", "")
	for _, tc := range []struct {
		expiredAgo  time.Duration
		wantRewrite bool
	}{{30 * time.Minute, false}, {2 * time.Hour, true}} {
		storeExpiredSnapshot(t, "codex", tc.expiredAgo)
		s := KnownSession([]string{"cheap", "strong"}).WithUsageQuota("codex", nil)
		d := Decision{Harness: "codex", Tier: TierSmall, Verdict: VerdictUnknown}
		plan := d.PlanForSession(CodexCaps, nil, s)
		if plan.RewriteModel != tc.wantRewrite {
			t.Errorf("expired %v ago: RewriteModel=%v (model %q), want %v", tc.expiredAgo, plan.RewriteModel, plan.Model.ID, tc.wantRewrite)
		}
	}
}
