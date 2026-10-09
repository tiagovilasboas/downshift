// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift/internal/quota"
)

func TestQuotaRequiredAndLegacyRemainExplicit(t *testing.T) {
	t.Setenv("DOWNSHIFT_QUOTA_FILE", filepath.Join(t.TempDir(), "absent"))
	s := KnownSession([]string{"candidate"}).WithUsageQuota("codex", nil)
	if !CanWriteSessionID("codex", "candidate", s, nil) || s.QuotaStatus("codex", "candidate") != quota.Unknown {
		t.Fatal("legacy mode should route without claiming quota")
	}
	t.Setenv("DOWNSHIFT_QUOTA_MODE", "required")
	s = s.WithUsageQuota("codex", nil)
	if CanWriteSessionID("codex", "candidate", s, nil) {
		t.Fatal("required quota allowed unknown")
	}
}

func TestQuotaFilteringPreservesOriginalQualityFloor(t *testing.T) {
	now := time.Now()
	zero, full := 0.0, 100.0
	s := KnownSession([]string{"cheap", "mid", "strong"})
	s = s.WithUsageQuota("codex", &quota.Snapshot{Version: 1, Harness: "codex", Source: "operator-normalized", ObservedAt: now.Add(-time.Second), ExpiresAt: now.Add(time.Minute), Windows: []quota.Window{
		{ID: "cheap-pool", Scope: "models", ModelIDs: []string{"cheap"}, UsedPercent: &zero, ResetsAt: now.Add(time.Hour)},
		{ID: "other-pool", Scope: "models", ModelIDs: []string{"mid", "strong"}, UsedPercent: &full, ResetsAt: now.Add(time.Hour)},
	}})
	for _, tier := range []Tier{TierMid, TierFrontier} {
		d := Decision{Harness: "codex", Tier: tier, Verdict: VerdictUnknown}
		plan := d.PlanForSession(CodexCaps, nil, s)
		if plan.RewriteModel {
			t.Fatalf("quota filtering demoted %s task to %s", tier, plan.Model.ID)
		}
	}
	d := Decision{Harness: "codex", Tier: TierSmall, Verdict: VerdictUnknown}
	plan := d.PlanForSession(CodexCaps, nil, s)
	if !plan.RewriteModel || plan.Model.ID != "cheap" {
		t.Fatalf("known funded cheap model not selected: %+v", plan)
	}
	if CanWriteSessionID("cursor", "cheap", s, nil) {
		t.Fatal("cross-harness budget leak")
	}
}

func TestNativeCursorExportFeedsHookQuotaGate(t *testing.T) {
	now := time.Now().UTC()
	path := filepath.Join(t.TempDir(), "cursor-usage.json")
	payload := map[string]any{
		"observed_at":        now,
		"plan_usage":         map[string]any{"auto_percent_used": 20.0},
		"billing_cycle_end":  now.Add(time.Hour).UnixMilli(),
		"auto_bucket_models": []string{"candidate"},
	}
	b, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b, 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DOWNSHIFT_CURSOR_NATIVE_FILE", path)
	t.Setenv("DOWNSHIFT_QUOTA_FILE", filepath.Join(t.TempDir(), "missing-quota.json"))
	s := KnownSession([]string{"candidate"}).WithUsageQuota("cursor", nil)
	if s.QuotaStatus("cursor", "candidate") != quota.Available || !CanWriteSessionID("cursor", "candidate", s, nil) {
		t.Fatalf("fresh native export did not authorize candidate: status=%s", s.QuotaStatus("cursor", "candidate"))
	}
	payload["observed_at"] = now.Add(-10 * time.Minute)
	b, err = json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b, 0600); err != nil {
		t.Fatal(err)
	}
	s = KnownSession([]string{"candidate"}).WithUsageQuota("cursor", nil)
	if s.QuotaStatus("cursor", "candidate") != quota.Stale || CanWriteSessionID("cursor", "candidate", s, nil) {
		t.Fatalf("stale native export did not hold candidate: status=%s", s.QuotaStatus("cursor", "candidate"))
	}
}
