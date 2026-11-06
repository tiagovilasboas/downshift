// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/tiagovilasboas/downshift/internal/accounting"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// printCompactionLead opens the report with the Compactor's work per harness,
// in tokens only. It is the first section because the Compactor runs on every
// harness and OS without a model-rewrite channel, so it carries no platform
// risk. It reuses the per-harness rows the exec log already produced; the
// routing figures of those rows are printed by the routed report.
func printCompactionLead(w io.Writer, rows []accounting.HarnessTokens, scope accounting.Scope) {
	fmt.Fprintf(w, "COMPACTION SAVINGS PER HARNESS (%s; tokens kept out of the context, per harness, never summed)\n", scope.Label())
	have := map[string]bool{}
	var seen bool
	for _, r := range rows {
		have[r.Harness] = true
		if r.CompactorAvoided > 0 {
			seen = true
		}
	}
	if !seen {
		fmt.Fprintln(w, "   No compaction events in this window.")
		if rest := harnessesWithoutEvents(have); len(rest) > 0 {
			fmt.Fprintf(w, "   no events: %s\n", strings.Join(rest, ", "))
		}
		return
	}
	fmt.Fprintf(w, "   %-14s %s\n", "harness", "tokens avoided (Compactor)")
	for _, r := range rows {
		if r.CompactorAvoided <= 0 {
			continue
		}
		fmt.Fprintf(w, "   %-14s %s\n", r.Harness, telemetry.ShortTokens(r.CompactorAvoided))
	}
	if rest := harnessesWithoutEvents(have); len(rest) > 0 {
		fmt.Fprintf(w, "   no events: %s\n", strings.Join(rest, ", "))
	}
}

// compactionRow is one harness's Compactor tokens for `report --json`.
type compactionRow struct {
	Harness          string `json:"harness"`
	CompactorAvoided int64  `json:"compactor_tokens_avoided"`
}

// compactionRows reads the Compactor column of the per-harness rows. The
// tokens_by_harness rows leave that column out, so the figure appears once.
func compactionRows(rows []accounting.HarnessTokens) []compactionRow {
	out := make([]compactionRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, compactionRow{Harness: r.Harness, CompactorAvoided: r.CompactorAvoided})
	}
	return out
}
