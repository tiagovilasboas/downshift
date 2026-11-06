// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package hint

import (
	"slices"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/routing/contract"
)

func TestParse(t *testing.T) {
	cases := []struct {
		name      string
		desc      string
		requested contract.Tier
		want      contract.Hint
	}{
		{"kind and effort", "[kind=code effort=deep] fix the parser", contract.TierFrontier,
			contract.Hint{Kind: contract.KindCode, Effort: contract.EffortDeep}},
		{"tier below requested is kept", "[kind=review tier=small]", contract.TierMid,
			contract.Hint{Kind: contract.KindReview, Tier: contract.TierSmall}},
		{"tier equal to requested is kept", "[tier=mid]", contract.TierMid,
			contract.Hint{Tier: contract.TierMid}},

		{"unknown kind and effort", "[kind=banana effort=max]", contract.TierFrontier, contract.Hint{}},
		{"unknown key", "[model=opus] task", contract.TierFrontier, contract.Hint{}},
		{"unknown tier name", "[tier=strong]", contract.TierFrontier, contract.Hint{}},
		{"mechanical is not a contract kind, effort kept", "[kind=mechanical effort=light]", contract.TierFrontier,
			contract.Hint{Effort: contract.EffortLight}},
		{"repeated key is ambiguous", "[kind=code kind=review]", contract.TierFrontier, contract.Hint{}},
		{"empty value", "[kind= effort=]", contract.TierFrontier, contract.Hint{}},
		{"empty prefix", "[] task", contract.TierFrontier, contract.Hint{}},
		{"no prefix", "kind=code fix it", contract.TierFrontier, contract.Hint{}},
		{"prefix not at the start", "fix it [kind=code]", contract.TierFrontier, contract.Hint{}},
		{"no closing bracket", "[kind=code effort=deep fix it", contract.TierFrontier, contract.Hint{}},
		{"empty description", "", contract.TierFrontier, contract.Hint{}},

		{"tier above requested is ignored", "[tier=frontier]", contract.TierSmall, contract.Hint{}},
		{"mid above small is ignored", "[tier=mid]", contract.TierSmall, contract.Hint{}},
		{"ignored tier keeps the other keys", "[kind=code tier=frontier effort=deep]", contract.TierMid,
			contract.Hint{Kind: contract.KindCode, Effort: contract.EffortDeep}},
		{"unknown requested tier keeps the model", "[tier=small]", contract.TierUnknown, contract.Hint{}},

		{"upper case prefix", "[KIND=CODE EFFORT=DEEP]", contract.TierFrontier,
			contract.Hint{Kind: contract.KindCode, Effort: contract.EffortDeep}},
		{"leading spaces and mixed case", "   [Kind=Docs]  write the readme", contract.TierFrontier,
			contract.Hint{Kind: contract.KindDocs}},
		{"spaces around equals and between pairs", "[ kind = code   effort =light ]", contract.TierFrontier,
			contract.Hint{Kind: contract.KindCode, Effort: contract.EffortLight}},
		{"leading newline and tab between pairs", "\n[kind=code\teffort=deep]", contract.TierFrontier,
			contract.Hint{Kind: contract.KindCode, Effort: contract.EffortDeep}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Parse(c.desc, c.requested); got != c.want {
				t.Errorf("Parse(%q, %q) = %+v, want %+v", c.desc, c.requested, got, c.want)
			}
		})
	}
}

// A tier hint never rises above the requested tier, for every pair of tiers.
func TestTierHintNeverRises(t *testing.T) {
	tiers := contract.Tiers()
	for _, requested := range append(slices.Clone(tiers), contract.TierUnknown) {
		for _, declared := range tiers {
			got := Parse("[tier="+string(declared)+"]", requested).Tier
			if got == contract.TierUnknown {
				continue
			}
			if !requested.Valid() || slices.Index(tiers, got) > slices.Index(tiers, requested) {
				t.Errorf("requested %q, declared %q: kept %q, a hint must not rise", requested, declared, got)
			}
		}
	}
}

func TestParseAgentType(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want string
	}{
		{"built-in type", "Explore", "Explore"},
		{"trimmed", "  general-purpose \n", "general-purpose"},
		{"dots and underscores", "mechanical_worker.v2", "mechanical_worker.v2"},
		{"longest allowed", strings.Repeat("a", 64), strings.Repeat("a", 64)},
		{"empty", "", ""},
		{"blank", "   ", ""},
		{"space inside is prose", "Explore the repo", ""},
		{"plugin scope is not a plain name", "plugin:agent", ""},
		{"path separator", "../agents/x", ""},
		{"too long", strings.Repeat("a", 65), ""},
		{"non-ascii letter", "Exploré", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ParseAgentType(c.raw); got != c.want {
				t.Errorf("ParseAgentType(%q) = %q, want %q", c.raw, got, c.want)
			}
		})
	}
}
