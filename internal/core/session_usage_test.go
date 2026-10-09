// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core

import (
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
