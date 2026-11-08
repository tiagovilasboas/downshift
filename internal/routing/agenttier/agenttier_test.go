// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package agenttier

import (
	"io/fs"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift-ai/agents"
	"github.com/tiagovilasboas/downshift-ai/internal/routing/contract"
)

// Every file in agents/ is a real agent and declares a valid tier (the README,
// routing matrix and protocols live in docs/agents). A new agent without a tier
// fails here.
func TestEveryAgentDeclaresAValidTier(t *testing.T) {
	declared := Declared(agents.FS)
	files, _ := fs.Glob(agents.FS, "*.md")
	for _, f := range files {
		name := strings.TrimSuffix(f, ".md")
		if !declared[name].Valid() {
			t.Errorf("%s declares no valid default_tier (want small, mid or frontier)", f)
		}
	}
	if len(declared) != 15 {
		t.Errorf("declared agents = %d, want 15", len(declared))
	}
}

func TestOfReadsTheDeclaredTiers(t *testing.T) {
	want := map[string]contract.Tier{
		"mechanical-worker": contract.TierSmall, "tester": contract.TierSmall,
		"researcher": contract.TierSmall, "performance": contract.TierSmall,
		"coder": contract.TierMid, "coder-back": contract.TierMid, "coder-front": contract.TierMid,
		"refactor": contract.TierMid, "documenter": contract.TierMid,
		"reviewer": contract.TierMid, "debugger": contract.TierMid, "platform": contract.TierMid,
		"architect": contract.TierFrontier, "ai": contract.TierFrontier, "security": contract.TierFrontier,
	}
	for name, tier := range want {
		if got := Of("downshift:"+name, "downshift:"); got != tier {
			t.Errorf("Of(%s) = %q, want %q", name, got, tier)
		}
	}
	if got := Of("coder", ""); got != contract.TierMid {
		t.Errorf("bare name with no prefix = %q, want mid", got)
	}
	if got := Of("coder-back", ""); got != contract.TierMid {
		t.Errorf("bare name coder-back with no prefix = %q, want mid", got)
	}
	if got := Of("coder-front", ""); got != contract.TierMid {
		t.Errorf("bare name coder-front with no prefix = %q, want mid", got)
	}
	for _, other := range []string{"", "coder", "other:coder", "downshift:", "downshift:nope", "Explore", "downshift:README"} {
		if got := Of(other, "downshift:"); got != contract.TierUnknown {
			t.Errorf("Of(%q) = %q, want unknown", other, got)
		}
	}
}
