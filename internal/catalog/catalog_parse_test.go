// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package catalog

import (
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/core"
)

// v0.1.0-beta.1 shipped "claude-haiku-4" as the small-tier id, and the release
// binary prints it. Anthropic has no such model: the Claude API ID is
// claude-haiku-4-5-20251001 with the alias claude-haiku-4-5
// (https://docs.anthropic.com/en/docs/about-claude/models). An alias may keep
// the old name so existing configs still match, but no entry may emit it.
func TestValidate_RejectsAliasOfAnotherHarnessCanonicalID(t *testing.T) {
	data := []byte(`{"entries":[
		{"id":"claude-haiku-4-5","harness":"claude-code","tier":"small","family":"claude-haiku"},
		{"id":"claude-4.5-haiku-thinking","aliases":["claude-haiku-4-5"],"harness":"cursor","tier":"small","family":"claude-4.5-haiku"}
	]}`)
	if _, err := parse(data); err == nil {
		t.Fatal("alias of another harness canonical id must be rejected")
	}
}

func TestParseTier(t *testing.T) {
	cases := []struct {
		s    string
		want core.Tier
		ok   bool
	}{
		{"small", core.TierSmall, true},
		{"mid", core.TierMid, true},
		{"frontier", core.TierFrontier, true},
		{"SMALL", core.TierSmall, true},
		{"unknown", 0, false},
		{"", 0, false},
	}
	for _, tc := range cases {
		got, ok := parseTier(tc.s)
		if ok != tc.ok {
			t.Errorf("parseTier(%q): ok=%v, want %v", tc.s, ok, tc.ok)
		}
		if ok && got != tc.want {
			t.Errorf("parseTier(%q) = %v, want %v", tc.s, got, tc.want)
		}
	}
}

func TestParse_RejectsDuplicateModelOrAlias(t *testing.T) {
	data := []byte(`{"entries":[{"id":"first","harness":"codex","tier":"small"},{"id":"second","aliases":["first"],"harness":"codex","tier":"mid"}]}`)
	if _, err := parse(data); err == nil {
		t.Fatal("parse accepted duplicate model ID/alias")
	}
}
