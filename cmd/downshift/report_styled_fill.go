// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"sort"

	"github.com/tiagovilasboas/downshift-ai/internal/console/reportview"
)

// styledFamilies is how many families a harness shows on the summary screen.
const styledFamilies = 3

// fillCompactor copies, for row's harness, the Compactor figures, the spawn
// verdicts, the top families and the quota text of the report into row. The
// figures are the report's own (summary_by_harness, compaction_detail,
// tokens_by_harness), so the screen and --json never disagree.
func fillCompactor(row *reportview.Row, d reportOut) {
	for _, p := range d.Summary {
		if p.Harness != row.Harness {
			continue
		}
		row.RawBytes, row.SavedBytes, row.RetrievedBytes, row.CutPct = p.RawBytes, p.SavedBytes, p.RetrievedBytes, p.CutPct
		row.Commands, row.Compacted, row.HookRuns, row.DirectRuns = p.Commands, p.Compacted, p.HookCommands, p.DirectCommands
		row.ModelSwitched, row.EffortChanged, row.Held, row.HeldGuardrail = p.ModelSwitched, p.EffortChanged, p.Held, p.HeldGuardrail
	}
	for _, h := range d.CompactionDetail.Harnesses {
		if h.Harness != row.Harness {
			continue
		}
		for _, f := range h.Families {
			if f.AvoidedTokens <= 0 {
				continue
			}
			cut := 0.0
			if f.RawTokens > 0 {
				cut = 100 * float64(f.AvoidedTokens) / float64(f.RawTokens)
			}
			row.Families = append(row.Families, reportview.Family{Name: f.Family, AvoidedTokens: f.AvoidedTokens, CutPct: cut})
		}
	}
	sort.SliceStable(row.Families, func(i, j int) bool { return row.Families[i].AvoidedTokens > row.Families[j].AvoidedTokens })
	if len(row.Families) > styledFamilies {
		row.Families = row.Families[:styledFamilies]
	}
	for _, e := range d.Efficiency.Harnesses {
		if e.Harness == row.Harness {
			row.Spawns = e.Spawns
		}
	}
	for _, t := range d.TokensByHarness {
		if t.Harness == row.Harness {
			if q := quotaText(t.Quota); q != "unknown" {
				row.Quota = q
			}
		}
	}
}
