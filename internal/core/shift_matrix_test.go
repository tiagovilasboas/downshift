// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core_test

import (
	"testing"

	"github.com/tiagovilasboas/downshift/internal/core"
)

// TestShiftMatrix checks the universal gear rule on the real catalog:
// downshift only when the current tier is above the task, upshift only when
// it is below, and an uncertain call must not rewrite the model.
func TestShiftMatrix(t *testing.T) {
	t.Setenv("DOWNSHIFT_MINILM", "0")
	cases := []struct {
		name       string
		prompt     string
		harness    string
		current    core.Tier
		complexity core.Complexity
		tier       core.Tier
		verdict    core.Verdict
	}{
		{"claude trivial on frontier downshifts", "rename the userId variable to user_id", "claude-code", core.TierFrontier, core.Trivial, core.TierSmall, core.VerdictDownshift},
		{"claude trivial on small stays", "rename the userId variable to user_id", "claude-code", core.TierSmall, core.Trivial, core.TierSmall, core.VerdictOK},
		{"claude simple on small stays", "add a field email to the User struct", "claude-code", core.TierSmall, core.Simple, core.TierSmall, core.VerdictOK},
		{"claude simple on mid downshifts", "add a field email to the User struct", "claude-code", core.TierMid, core.Simple, core.TierSmall, core.VerdictDownshift},
		{"claude cube on small stays", "implement a three.js scene with a rotating cube", "claude-code", core.TierSmall, core.Simple, core.TierSmall, core.VerdictOK},
		{"claude medium on small upshifts", "implement the feature to export sales as CSV", "claude-code", core.TierSmall, core.Medium, core.TierMid, core.VerdictUpshift},
		{"claude medium on mid stays", "implement the feature to export sales as CSV", "claude-code", core.TierMid, core.Medium, core.TierMid, core.VerdictOK},
		{"claude medium on frontier downshifts", "implement the feature to export sales as CSV", "claude-code", core.TierFrontier, core.Medium, core.TierMid, core.VerdictDownshift},
		{"claude complex on small upshifts", "rearchitect the auth module to support multi-tenant organisations", "claude-code", core.TierSmall, core.Complex, core.TierFrontier, core.VerdictUpshift},
		{"claude complex on frontier stays", "rearchitect the auth module to support multi-tenant organisations", "claude-code", core.TierFrontier, core.Complex, core.TierFrontier, core.VerdictOK},
		{"codex trivial on frontier downshifts", "rename the userId variable to user_id", "codex", core.TierFrontier, core.Trivial, core.TierSmall, core.VerdictDownshift},
		{"codex cube on small stays", "implement a three.js scene with a rotating cube", "codex", core.TierSmall, core.Simple, core.TierSmall, core.VerdictOK},
		{"codex medium on mid stays", "implement the feature to export sales as CSV", "codex", core.TierMid, core.Medium, core.TierMid, core.VerdictOK},
		{"codex complex on small upshifts to frontier", "rearchitect the auth module to support multi-tenant organisations", "codex", core.TierSmall, core.Complex, core.TierFrontier, core.VerdictUpshift},
		{"codex vague on small does not rewrite", "help me with this", "codex", core.TierSmall, core.Medium, core.TierMid, core.VerdictUpshift},
		{"codex vague on frontier does not rewrite", "help me with this", "codex", core.TierFrontier, core.Medium, core.TierMid, core.VerdictDownshift},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			currentID := catID(tc.harness, tc.current)
			d := core.Route(tc.prompt, tc.harness, currentID, cat)
			if d.Complexity != tc.complexity {
				t.Fatalf("complexity = %s, want %s", d.Complexity, tc.complexity)
			}
			if d.Tier != tc.tier {
				t.Fatalf("tier = %s, want %s", d.Tier, tc.tier)
			}
			if d.Verdict != tc.verdict {
				t.Fatalf("verdict = %s, want %s", d.Verdict, tc.verdict)
			}
			wantID := catID(tc.harness, tc.tier)
			if d.Model.ID != wantID {
				t.Fatalf("recommended = %s, want %s", d.Model.ID, wantID)
			}
			rewrite := d.ShouldRewriteModel()
			switch d.Verdict {
			case core.VerdictOK:
				if rewrite {
					t.Fatal("same-tier decision must not rewrite")
				}
			case core.VerdictUpshift, core.VerdictDownshift:
				if rewrite != d.Confident {
					t.Fatalf("rewrite = %v, confident = %v; uncertain shifts must keep the current model", rewrite, d.Confident)
				}
			}
			if tc.prompt == "help me with this" && rewrite {
				t.Fatal("vague prompt must not rewrite")
			}
		})
	}
}
