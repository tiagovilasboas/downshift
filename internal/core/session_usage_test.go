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

// A lower-priority healthy observation must never reopen a higher-priority
// exhausted, stale, or malformed source.
func TestUsageQuotaSourcePrecedenceDoesNotReopenCredit(t *testing.T) {
	now := time.Now().UTC()
	for _, tc := range []struct {
		name          string
		cacheUsed     float64
		nativeUsed    float64
		nativePresent bool
		nativeInvalid bool
		nativeStale   bool
		hookUsed      *float64
		hookInvalid   bool
		want          quota.Status
		wantSource    string
	}{
		{name: "hook healthy overrides exhausted native and cache", cacheUsed: 100, nativeUsed: 100, nativePresent: true, hookUsed: quotaNumber(20), want: quota.Available, wantSource: "cursor-usage"},
		{name: "hook exhaustion vetoes healthy native and cache", nativeUsed: 20, nativePresent: true, hookUsed: quotaNumber(100), want: quota.Exhausted, wantSource: "cursor-usage"},
		{name: "invalid hook vetoes healthy native and cache", nativeUsed: 20, nativePresent: true, hookInvalid: true, want: quota.Unknown, wantSource: "cursor-usage"},
		{name: "native exhaustion vetoes healthy cache", nativeUsed: 100, nativePresent: true, want: quota.Exhausted, wantSource: "cursor-usage"},
		{name: "healthy native overrides exhausted cache", cacheUsed: 100, nativeUsed: 20, nativePresent: true, want: quota.Available, wantSource: "cursor-usage"},
		{name: "malformed native vetoes healthy cache", nativePresent: true, nativeInvalid: true, want: quota.Unknown},
		{name: "stale native vetoes healthy cache", nativeUsed: 20, nativePresent: true, nativeStale: true, want: quota.Stale, wantSource: "cursor-usage"},
		{name: "absent native uses cache", cacheUsed: 20, want: quota.Available, wantSource: "operator-normalized"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("DOWNSHIFT_QUOTA_MODE", "")
			dir := t.TempDir()
			cachePath := filepath.Join(dir, "quota.json")
			t.Setenv("DOWNSHIFT_QUOTA_FILE", cachePath)
			cache := quota.Snapshot{Version: 1, Harness: "cursor", Source: "operator-normalized", ObservedAt: now.Add(-time.Second), ExpiresAt: now.Add(time.Minute), Windows: []quota.Window{{ID: "shared", Scope: "harness", UsedPercent: &tc.cacheUsed, ResetsAt: now.Add(time.Hour)}}}
			if err := quota.Store(cachePath, cache); err != nil {
				t.Fatal(err)
			}
			t.Setenv("DOWNSHIFT_CURSOR_NATIVE_FILE", "")
			if tc.nativePresent {
				observed := now.Add(-time.Second)
				if tc.nativeStale {
					observed = now.Add(-10 * time.Minute)
				}
				payload, err := json.Marshal(map[string]any{"observed_at": observed, "plan_usage": map[string]any{"auto_percent_used": tc.nativeUsed}, "billing_cycle_end": now.Add(time.Hour).UnixMilli(), "auto_bucket_models": []string{"candidate"}})
				if err != nil {
					t.Fatal(err)
				}
				if tc.nativeInvalid {
					payload = []byte(`{"plan_usage":`)
				}
				nativePath := filepath.Join(dir, "cursor-usage.json")
				if err := os.WriteFile(nativePath, payload, 0600); err != nil {
					t.Fatal(err)
				}
				t.Setenv("DOWNSHIFT_CURSOR_NATIVE_FILE", nativePath)
			}
			var supplied *quota.Snapshot
			if tc.hookUsed != nil || tc.hookInvalid {
				hook := cache
				hook.Source = "cursor-usage"
				hook.Windows = []quota.Window{{ID: "hook-shared", Scope: "harness", UsedPercent: tc.hookUsed, ResetsAt: now.Add(time.Hour)}}
				supplied = &hook
			}
			session := KnownSession([]string{"candidate"}).WithUsageQuota("cursor", supplied)
			if got := session.QuotaStatus("cursor", "candidate"); got != tc.want {
				t.Fatalf("quota status=%s, want %s", got, tc.want)
			}
			if got := CanWriteSessionID("cursor", "candidate", session, nil); got != (tc.want == quota.Available) {
				t.Fatalf("write allowed=%v with quota %s", got, tc.want)
			}
			gotSource := ""
			if session.Usage != nil {
				gotSource = session.Usage.Source
			}
			if gotSource != tc.wantSource {
				t.Fatalf("source=%q, want %q", gotSource, tc.wantSource)
			}
		})
	}
}

func quotaNumber(v float64) *float64 { return &v }

func TestNativeDownshiftDoesNotBecomeCapabilityOrPriceUpshift(t *testing.T) {
	for _, tc := range []struct {
		name       string
		targetTier Tier
		targetCost float64
		wantWrite  bool
	}{
		{"only frontier funded", TierFrontier, 3, false},
		{"same tier more expensive", TierMid, 4, false},
		{"same tier cheaper", TierMid, 1, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			now := time.Now().UTC()
			current := Model{ID: "qa-current", Harness: "cursor", Tier: TierMid, OutputM: 2}
			candidate := Model{ID: "qa-candidate", Harness: "cursor", Tier: tc.targetTier, OutputM: tc.targetCost}
			resolver := nativeResolver{models: map[string]Model{current.ID: current, candidate.ID: candidate}}
			session := KnownSession([]string{candidate.ID, current.ID}).WithUsageQuota("cursor", &quota.Snapshot{
				Version: 1, Harness: "cursor", Source: "cursor-usage", ObservedAt: now.Add(-time.Second), ExpiresAt: now.Add(time.Minute),
				Windows: []quota.Window{
					{ID: "available", Scope: "models", ModelIDs: []string{candidate.ID}, UsedPercent: quotaNumber(20), ResetsAt: now.Add(time.Hour)},
					{ID: "exhausted", Scope: "models", ModelIDs: []string{current.ID}, UsedPercent: quotaNumber(100), ResetsAt: now.Add(time.Hour)},
				},
			})
			session.NativeAvailability = true
			decision := Decision{Harness: "cursor", Tier: TierSmall, Verdict: VerdictDownshift, CurrentModel: current, RequestedID: current.ID}
			plan := decision.PlanForSession(CursorCaps, resolver, session)
			if plan.RewriteModel != tc.wantWrite {
				t.Fatalf("downshift plan=%+v, want rewrite=%v", plan, tc.wantWrite)
			}
			if tc.wantWrite && plan.Model.ID != candidate.ID {
				t.Fatalf("funded cheaper target not selected: %+v", plan)
			}
		})
	}
}

func TestNativeUpshiftExplicitFlagKeepsOrdinaryFundedFallback(t *testing.T) {
	t.Setenv("DOWNSHIFT_SESSION_MODELS", filepath.Join(t.TempDir(), "absent.json"))
	cheap := Model{ID: "qa-cheap", Harness: "cursor", Tier: TierSmall, OutputM: 1}
	middle := Model{ID: "qa-middle", Harness: "cursor", Tier: TierMid, OutputM: 2}
	explicit := Model{ID: "qa-explicit", Harness: "cursor", Tier: TierFrontier, OutputM: 3}
	resolver := nativeResolver{
		models:   map[string]Model{cheap.ID: cheap, middle.ID: middle, explicit.ID: explicit},
		explicit: map[string]bool{explicit.ID: true},
	}
	for _, tc := range []struct {
		name, flag, want string
	}{
		{"off selects ordinary funded mid", "", middle.ID},
		{"on allows explicit funded frontier", "1", explicit.ID},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("DOWNSHIFT_EXPLICIT_UPSHIFT", tc.flag)
			now := time.Now().UTC()
			session := KnownSession([]string{explicit.ID, middle.ID, cheap.ID}).WithUsageQuota("cursor", &quota.Snapshot{
				Version: 1, Harness: "cursor", Source: "cursor-usage", ObservedAt: now.Add(-time.Second), ExpiresAt: now.Add(time.Minute),
				Windows: []quota.Window{{ID: "shared", Scope: "harness", UsedPercent: quotaNumber(20), ResetsAt: now.Add(time.Hour)}},
			})
			session.NativeAvailability = true
			decision := Decision{Harness: "cursor", Tier: TierMid, Verdict: VerdictUpshift, CurrentModel: cheap, RequestedID: cheap.ID}
			plan := decision.PlanForSession(CursorCaps, resolver, session)
			if !plan.RewriteModel || plan.Model.ID != tc.want {
				t.Fatalf("explicit flag=%q plan=%+v, want funded rewrite to %s", tc.flag, plan, tc.want)
			}
		})
	}
}

// A stored claude-statusline snapshot is the cache WithUsageQuota loads when
// the hook sends no usage_quota. Context-window utilization is not that snapshot.
func TestStoredClaudeStatuslineSnapshotHoldsClaudeRewrite(t *testing.T) {
	t.Setenv("DOWNSHIFT_QUOTA_MODE", "")
	t.Setenv("DOWNSHIFT_CURSOR_NATIVE_FILE", "")
	cheap := Model{ID: "claude-haiku-4-5", Native: "haiku", Harness: "claude-code", Tier: TierSmall}
	strong := Model{ID: "claude-opus-5-5", Native: "opus", Harness: "claude-code", Tier: TierFrontier}
	resolver := nativeResolver{models: map[string]Model{cheap.ID: cheap, strong.ID: strong}}
	decision := Decision{
		Harness:      "claude-code",
		Tier:         TierSmall,
		Verdict:      VerdictDownshift,
		CurrentModel: strong,
		RequestedID:  strong.ID,
	}
	now := time.Now().UTC().Add(-time.Second)
	reset := now.Add(time.Hour).Unix()
	rateLimits := func(used float64) map[string]any {
		window := map[string]any{"used_percentage": used, "resets_at": reset}
		return map[string]any{"five_hour": window, "seven_day": window}
	}
	for _, tc := range []struct {
		name         string
		payload      map[string]any
		wantStatus   quota.Status
		wantRewrite  bool
		wantHeld     bool
		wantCanWrite bool
	}{
		{
			name:         "high used percentage holds rewrite",
			payload:      map[string]any{"context_window": map[string]any{"used_percentage": 0}, "rate_limits": rateLimits(100)},
			wantStatus:   quota.Exhausted,
			wantRewrite:  false,
			wantHeld:     true,
			wantCanWrite: false,
		},
		{
			name:         "low used percentage can be available",
			payload:      map[string]any{"context_window": map[string]any{"used_percentage": 100}, "rate_limits": rateLimits(20)},
			wantStatus:   quota.Available,
			wantRewrite:  true,
			wantHeld:     false,
			wantCanWrite: true,
		},
		{
			name:         "context window only does not authorize",
			payload:      map[string]any{"context_window": map[string]any{"used_percentage": 0}},
			wantStatus:   quota.Unknown,
			wantRewrite:  false,
			wantHeld:     true,
			wantCanWrite: false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := json.Marshal(tc.payload)
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(t.TempDir(), "quota.json")
			t.Setenv("DOWNSHIFT_QUOTA_FILE", path)
			snap, err := quota.ParseNative("claude-statusline", raw, now, 5*time.Minute)
			if err != nil {
				if err := os.WriteFile(path, raw, 0600); err != nil {
					t.Fatal(err)
				}
			} else if err := quota.Store(path, snap); err != nil {
				t.Fatal(err)
			}
			session := KnownSession([]string{cheap.ID, strong.ID}).WithUsageQuota("claude-code", nil)
			if got := session.QuotaStatus("claude-code", cheap.ID); got != tc.wantStatus {
				t.Fatalf("quota status=%s, want %s", got, tc.wantStatus)
			}
			if tc.wantStatus == quota.Available && (session.Usage == nil || session.Usage.Source != "claude-statusline") {
				t.Fatalf("available rewrite did not load stored claude-statusline snapshot: %+v", session.Usage)
			}
			plan := decision.PlanForSession(ClaudeCodeCaps, resolver, session)
			if plan.RewriteModel != tc.wantRewrite || plan.CreditHeld != tc.wantHeld {
				t.Fatalf("plan=%+v, want rewrite=%v held=%v", plan, tc.wantRewrite, tc.wantHeld)
			}
			if tc.wantRewrite && plan.Model.ID != cheap.ID {
				t.Fatalf("funded rewrite target=%q, want %s", plan.Model.ID, cheap.ID)
			}
			if got := CanWriteSessionID("claude-code", cheap.ID, session, resolver); got != tc.wantCanWrite {
				t.Fatalf("CanWriteSessionID=%v, want %v", got, tc.wantCanWrite)
			}
		})
	}
}
