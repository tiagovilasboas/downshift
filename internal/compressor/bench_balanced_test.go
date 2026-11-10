// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor_test

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/compressor"
	"github.com/tiagovilasboas/downshift-ai/internal/execwrap"
)

// balancedDir holds large real outputs of this repository (author lines
// scrubbed): the inputs balanced compaction is for. The file prefix names the
// shape: log_, diff_ or matches_.
const balancedDir = "testdata/balanced"

// balancedRow is one large fixture: raw, after the lossless compressor, and
// after the balanced cut, each with the marker the runner appends.
type balancedRow struct {
	name                    string
	raw, lossless, balanced int
}

func balancedKind(name string) compressor.BalanceKind {
	switch {
	case strings.HasPrefix(name, "diff_"):
		return compressor.BalanceDiff
	case strings.HasPrefix(name, "matches_"):
		return compressor.BalanceMatches
	}
	return compressor.BalanceLog
}

// balancedRows replays each fixture the way the runner does: the lossless
// compressor first, then, over execwrap.BalanceThreshold, the balanced cut.
func balancedRows() []balancedRow {
	entries, _ := os.ReadDir(balancedDir)
	var rows []balancedRow
	for _, e := range entries {
		in, err := os.ReadFile(filepath.Join(balancedDir, e.Name()))
		if err != nil {
			continue
		}
		r := balancedRow{name: strings.TrimSuffix(e.Name(), ".txt"), raw: len(in), lossless: len(in)}
		shown := in
		if res := compressor.CompressExit(in, compressor.ModeSafe, 0); res.Applied && len(res.Output)+len(execwrap.Marker(len(in), len(res.Output))) < len(in) {
			shown = res.Output
			r.lossless = len(res.Output) + len(execwrap.Marker(len(in), len(res.Output)))
		}
		r.balanced = r.lossless
		if len(shown) > execwrap.BalanceThreshold {
			if cut, ok := compressor.Balance(balancedKind(e.Name()), shown); ok {
				r.balanced = len(cut) + len(execwrap.MarkerWithID(len(in), len(cut), "0123456789ab"))
			}
		}
		rows = append(rows, r)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].name < rows[j].name })
	return rows
}

func balancedSection(rows []balancedRow) string {
	var b strings.Builder
	b.WriteString("\n## Balanced: large outputs\n\n")
	b.WriteString("Balanced compaction (on by default) cuts an output over 16 KB once CCR stored its raw output, so every cut line stays retrievable. These fixtures are real outputs of this repository. `Lossless` is the compressor alone; `Balanced` adds the cut. Bytes include the marker line.\n\n")
	b.WriteString("| Fixture | Raw bytes | Lossless bytes | Lossless saving % | Balanced bytes | Balanced saving % |\n|---|--:|--:|--:|--:|--:|\n")
	var raw, lossless, balanced int
	for _, r := range rows {
		fmt.Fprintf(&b, "| %s | %d | %d | %.1f | %d | %.1f |\n", r.name, r.raw, r.lossless, savingPct(r.raw, r.lossless), r.balanced, savingPct(r.raw, r.balanced))
		raw, lossless, balanced = raw+r.raw, lossless+r.lossless, balanced+r.balanced
	}
	fmt.Fprintf(&b, "| TOTAL | %d | %d | %.1f | %d | %.1f |\n", raw, lossless, savingPct(raw, lossless), balanced, savingPct(raw, balanced))
	return b.String()
}

// TestBalancedFixturesCut holds the floor: each large fixture loses at least
// half its bytes with balanced compaction.
func TestBalancedFixturesCut(t *testing.T) {
	rows := balancedRows()
	if len(rows) < 4 {
		t.Fatalf("balanced fixtures = %d, want 4", len(rows))
	}
	for _, r := range rows {
		if r.balanced*2 > r.raw {
			t.Errorf("%s: balanced %d of %d raw bytes, want under half", r.name, r.balanced, r.raw)
		}
	}
}
