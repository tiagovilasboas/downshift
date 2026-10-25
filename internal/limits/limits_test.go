// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package limits

import (
	"testing"
)

func TestForHarness(t *testing.T) {
	cases := []struct {
		harness string
		wantNil bool
	}{
		{"kirocrew", false},
		{"claudecode", false},
		{"cursor", false},
		{"codex", false},
		{"antigravity", false},
		{"opencode", false},
		{"unknown", true},
	}
	for _, tc := range cases {
		t.Run(tc.harness, func(t *testing.T) {
			got := ForHarness(tc.harness)
			if tc.wantNil && got != nil {
				t.Errorf("ForHarness(%q) = %v; want nil", tc.harness, got)
			}
			if !tc.wantNil && got == nil {
				t.Errorf("ForHarness(%q) = nil; want non-nil", tc.harness)
			}
		})
	}
}

func TestHasFullSupport(t *testing.T) {
	cases := []struct {
		harness string
		feature string
		want    bool
	}{
		{"kirocrew", "model_rewrite", false},
		{"kirocrew", "token_counts", false},
		{"claudecode", "model_rewrite", true},
		{"claudecode", "token_counts", true},
		{"cursor", "model_rewrite", false}, // partial
	}
	for _, tc := range cases {
		t.Run(tc.harness+"_"+tc.feature, func(t *testing.T) {
			h := ForHarness(tc.harness)
			got := h.HasFullSupport(tc.feature)
			if got != tc.want {
				t.Errorf("HasFullSupport(%q) = %v; want %v", tc.feature, got, tc.want)
			}
		})
	}
}

func TestUnsupportedFeatures(t *testing.T) {
	// KiroCrew should have multiple unsupported features
	kiro := ForHarness("kirocrew")
	unsupported := kiro.UnsupportedFeatures()
	if len(unsupported) == 0 {
		t.Error("KiroCrew should have unsupported features")
	}

	// Check specific limitations exist
	foundModelRewrite := false
	foundTokenCounts := false
	for _, lim := range unsupported {
		if lim.Feature == "model_rewrite" {
			foundModelRewrite = true
		}
		if lim.Feature == "token_counts" {
			foundTokenCounts = true
		}
	}
	if !foundModelRewrite {
		t.Error("KiroCrew missing model_rewrite limitation")
	}
	if !foundTokenCounts {
		t.Error("KiroCrew missing token_counts limitation")
	}

	// Claude Code should have full support (no unsupported)
	cc := ForHarness("claudecode")
	ccUnsupported := cc.UnsupportedFeatures()
	if len(ccUnsupported) != 0 {
		t.Errorf("Claude Code has %d unsupported features; want 0", len(ccUnsupported))
	}
}

func TestGetLimitation(t *testing.T) {
	kiro := ForHarness("kirocrew")
	lim := kiro.GetLimitation("token_counts")
	if lim == nil {
		t.Fatal("GetLimitation(token_counts) = nil")
	}
	if lim.Status != "none" {
		t.Errorf("token_counts status = %q; want 'none'", lim.Status)
	}
	if lim.Reason == "" {
		t.Error("token_counts reason is empty")
	}
}

func TestSummaryLine(t *testing.T) {
	kiro := ForHarness("kirocrew")
	summary := kiro.SummaryLine()
	if summary == "" {
		t.Error("SummaryLine() is empty")
	}
	if summary == "full support" {
		t.Error("KiroCrew should not report full support")
	}

	cc := ForHarness("claudecode")
	ccSummary := cc.SummaryLine()
	if ccSummary != "full support" {
		t.Errorf("Claude Code summary = %q; want 'full support'", ccSummary)
	}
}

func TestAll(t *testing.T) {
	all := All()
	if len(all) < 6 {
		t.Errorf("All() returned %d harnesses; want at least 6", len(all))
	}
	expected := []string{"kirocrew", "claudecode", "cursor", "codex", "antigravity", "opencode"}
	for _, h := range expected {
		if _, ok := all[h]; !ok {
			t.Errorf("All() missing harness %q", h)
		}
	}
}
