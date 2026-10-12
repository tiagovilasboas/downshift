// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/core"
)

// promptCase is one hand-labelled prompt. Labels are judgement calls about the
// right-sized tier, written WITHOUT running the router, so the corpus measures
// the classifier instead of echoing it. Effort follows the tier (EffortFor).
type promptCase struct {
	ID         string `json:"id"`
	Prompt     string `json:"prompt"`
	WantTier   string `json:"want_tier"`             // small|mid|frontier
	WantEffort string `json:"want_effort,omitempty"` // low|medium|high; derived from tier when empty
	// WantHold, when true, asserts that with no current model known the model
	// is NOT rewritten (doubtful or no-signal prompt). Omit to not assert.
	WantHold *bool `json:"want_hold,omitempty"`
	// KnownGap marks a label the router currently misses. The case is reported
	// but does not fail the build; remove the flag when the logic is fixed.
	KnownGap bool   `json:"known_gap,omitempty"`
	Note     string `json:"note,omitempty"`
}

func loadPromptCorpus(t *testing.T) map[string][]promptCase {
	t.Helper()
	files, err := filepath.Glob("testdata/corpus/*.json")
	if err != nil || len(files) == 0 {
		t.Skip("no corpus files")
	}
	sort.Strings(files)
	out := map[string][]promptCase{}
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		var cs []promptCase
		if err := json.Unmarshal(b, &cs); err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		out[filepath.Base(f)] = cs
	}
	return out
}

func TestPromptCorpusFiles(t *testing.T) {
	var total, miss, gap, wrongEffort int
	for file, cases := range loadPromptCorpus(t) {
		seen := map[string]bool{}
		for _, c := range cases {
			if c.ID == "" || c.Prompt == "" || c.WantTier == "" {
				t.Errorf("%s: case %q is incomplete", file, c.ID)
				continue
			}
			if seen[c.ID] {
				t.Errorf("%s: duplicate id %s", file, c.ID)
			}
			seen[c.ID] = true
			total++
			d := core.Route(c.Prompt, "claude-code", "", cat)
			if d.Tier.String() != c.WantTier {
				if c.KnownGap {
					gap++
					t.Logf("known gap %s/%s: want %s got %s (%s)", file, c.ID, c.WantTier, d.Tier, d.Complexity)
				} else {
					miss++
					t.Errorf("%s/%s: want tier %s, got %s (complexity %s, confident=%v)\n   prompt: %.140s", file, c.ID, c.WantTier, d.Tier, d.Complexity, d.Confident, c.Prompt)
				}
				continue
			}
			wantEffort := c.WantEffort
			if wantEffort == "" {
				wantEffort = core.EffortFor(d.Tier).String()
			}
			if d.Effort.String() != wantEffort {
				wrongEffort++
				t.Errorf("%s/%s: want effort %s, got %s", file, c.ID, wantEffort, d.Effort)
			}
			if c.WantHold != nil && *c.WantHold == d.ShouldRewriteModel() && !c.KnownGap {
				t.Errorf("%s/%s: want_hold=%v but ShouldRewriteModel()=%v (confident=%v)", file, c.ID, *c.WantHold, d.ShouldRewriteModel(), d.Confident)
			}
		}
	}
	t.Logf("corpus: %d cases, %d tier misses, %d known gaps, %d effort mismatches", total, miss, gap, wrongEffort)
}
