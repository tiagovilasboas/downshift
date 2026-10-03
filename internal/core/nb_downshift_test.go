// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: BUSL-1.1
// Commercial use requires a licence — see LICENSE for terms.

package core_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/tiagovilasboas/harness-downshift/internal/core"
)

// loadTrivialWould returns, per golden row (same order as loadNBGolden), whether
// the Python reference expects the NB TRIVIAL downshift opinion to fire.
func loadTrivialWould(t *testing.T) []bool {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "nbtier", "testdata", "trivial_golden.json"))
	if err != nil {
		t.Fatal(err)
	}
	var rows []struct{ Would bool }
	if err := json.Unmarshal(b, &rows); err != nil {
		t.Fatal(err)
	}
	out := make([]bool, len(rows))
	for i, r := range rows {
		out[i] = r.Would
	}
	return out
}

// Flag OFF (default): routing is identical to main on seed + holdout and the
// would-be downshift is recorded in shadow mode for exactly the reference set.
func TestNBDownshift_OffKeepsRoutingAndRecordsShadow(t *testing.T) {
	offlineRouting(t)
	t.Setenv(core.NBUpshiftEnv, "")
	t.Setenv(core.NBDownshiftEnv, "")
	golden, would := loadNBGolden(t), loadTrivialWould(t)
	if len(golden) != len(would) {
		t.Fatalf("golden rows %d vs %d", len(golden), len(would))
	}
	shadow := 0
	for i, g := range golden {
		d := core.Route(g.Prompt, "claude-code", "claude-opus-4-8", cat)
		if d.Tier.String() != g.Prod {
			t.Errorf("flag off changed routing: tier %s, main %s for %q", d.Tier, g.Prod, g.Prompt)
		}
		if d.NBDownshift.Applied {
			t.Errorf("flag off applied a downshift for %q", g.Prompt)
		}
		if d.NBDownshift.Would != would[i] {
			t.Errorf("shadow would=%v, want %v for %q", d.NBDownshift.Would, would[i], g.Prompt)
		}
		if d.NBDownshift.Would {
			shadow++
			if d.Tier != core.TierMid || d.RiskFloor || d.NBDownshift.Margin < 0.2 {
				t.Errorf("shadow opinion outside its gate: tier=%s riskFloor=%v %+v for %q", d.Tier, d.RiskFloor, d.NBDownshift, g.Prompt)
			}
		}
	}
	if shadow != 21 { // docs/design/router-generalization.md, NBD-T on seed
		t.Errorf("shadow downshifts = %d, want 21", shadow)
	}
}

// Flag ON: exactly the reference set moves mid -> small, nothing else moves,
// no frontier task reaches small, and the rewrite is allowed for the applied
// downshifts only.
func TestNBDownshift_OnAppliesTrivialOnly(t *testing.T) {
	offlineRouting(t)
	t.Setenv(core.NBUpshiftEnv, "")
	t.Setenv(core.NBDownshiftEnv, "1")
	golden, would := loadNBGolden(t), loadTrivialWould(t)
	applied := 0
	for i, g := range golden {
		d := core.Route(g.Prompt, "claude-code", "claude-opus-4-8", cat)
		want := g.Prod
		if would[i] {
			want = "small"
		}
		if d.Tier.String() != want {
			t.Errorf("flag on: tier %s, want %s for %q", d.Tier, want, g.Prompt)
		}
		if g.Gold == "COMPLEX" && d.Tier == core.TierSmall {
			t.Errorf("FRONTIER->SMALL for %q", g.Prompt)
		}
		if d.NBDownshift.Applied {
			applied++
			if d.RiskFloor || d.Complexity != core.Trivial {
				t.Errorf("applied downshift: riskFloor=%v complexity=%s for %q", d.RiskFloor, d.Complexity, g.Prompt)
			}
			if !d.ShouldRewriteModel() || d.SafeVerdict != core.VerdictDownshift {
				t.Errorf("applied downshift not rewritten: safe=%s corrections=%v for %q", d.SafeVerdict, d.Corrections, g.Prompt)
			}
		}
	}
	if applied != 21 {
		t.Errorf("applied downshifts = %d, want 21", applied)
	}
}
