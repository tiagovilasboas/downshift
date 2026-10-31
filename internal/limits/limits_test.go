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
		{"claude-code", false},
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
		{"claude-code", "model_rewrite", true},
		{"claude-code", "token_counts", true},
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

	// Claude Code: every feature is full except effort, which the Agent tool
	// cannot take per call (evidence 2026-10-31).
	cc := ForHarness("claude-code")
	ccUnsupported := cc.UnsupportedFeatures()
	if len(ccUnsupported) != 1 || ccUnsupported[0].Feature != FeatureEffort || ccUnsupported[0].Status != StatusNone {
		t.Errorf("Claude Code unsupported = %v; want only effort:none", ccUnsupported)
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

	cc := ForHarness("claude-code")
	ccSummary := cc.SummaryLine()
	if ccSummary != "effort:none" {
		t.Errorf("Claude Code summary = %q; want 'effort:none'", ccSummary)
	}
}

func TestAll(t *testing.T) {
	all := All()
	if len(all) < 6 {
		t.Errorf("All() returned %d harnesses; want at least 6", len(all))
	}
	expected := []string{"kirocrew", "claude-code", "cursor", "codex", "antigravity", "opencode"}
	for _, h := range expected {
		if _, ok := all[h]; !ok {
			t.Errorf("All() missing harness %q", h)
		}
	}
}

func TestClaudeDesktopSurfaces(t *testing.T) {
	// Chat: three features are none because there is no model-carrying spawn
	// and no shell tool; MCP is partial because the read-only tools answer.
	chat := ForHarness("claude-desktop-chat")
	if chat == nil {
		t.Fatal("ForHarness(claude-desktop-chat) = nil; want non-nil")
	}
	for _, feature := range []string{FeatureModelRewrite, FeatureCompaction, FeatureMidSessionRouting} {
		lim := chat.GetLimitation(feature)
		if lim == nil {
			t.Errorf("claude-desktop-chat has no %s limitation", feature)
			continue
		}
		if lim.Status != StatusNone {
			t.Errorf("claude-desktop-chat %s = %q; want %q", feature, lim.Status, StatusNone)
		}
	}
	mcp := chat.GetLimitation(FeatureMCP)
	if mcp == nil || mcp.Status != StatusPartial {
		t.Errorf("claude-desktop-chat mcp = %v; want %q", mcp, StatusPartial)
	}

	// Cowork: every hook-driven feature is none because the binary and hooks
	// never reach the guest VM. MCP was neither seen working nor failing, so it
	// is unverified, not none: declaring none would claim evidence we lack.
	cowork := ForHarness("claude-desktop-cowork")
	if cowork == nil {
		t.Fatal("ForHarness(claude-desktop-cowork) = nil; want non-nil")
	}
	for _, lim := range cowork.Limitations {
		want := StatusNone
		if lim.Feature == FeatureMCP {
			want = StatusUnverified
		}
		if lim.Status != want {
			t.Errorf("claude-desktop-cowork %s = %q; want %q", lim.Feature, lim.Status, want)
		}
		if lim.Reason == "" {
			t.Errorf("claude-desktop-cowork %s has an empty reason", lim.Feature)
		}
	}
	if cowork.GetLimitation(FeatureMCP) == nil {
		t.Error("claude-desktop-cowork has no mcp limitation")
	}

	// Both surfaces are in All(), which the matrix parity test requires.
	all := All()
	for _, id := range []string{"claude-desktop-chat", "claude-desktop-cowork"} {
		if _, ok := all[id]; !ok {
			t.Errorf("All() missing surface %q", id)
		}
	}
}
