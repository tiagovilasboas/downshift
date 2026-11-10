// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"github.com/tiagovilasboas/downshift-ai/internal/accounting"
)

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
