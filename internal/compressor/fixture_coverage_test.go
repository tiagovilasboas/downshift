// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor_test

// Fixture coverage report (informational). For each compressor Format and each
// execwrap Family it counts the clean and the preserve fixtures in
// testdata/corpus/MANIFEST and marks each count 0, 1 or 2+. It fails nothing
// about the numbers; the test only asserts that every Format and Family is
// listed. Print it with:
//
//	go test ./internal/compressor -run FixtureCoverage -v

import (
	"fmt"
	"sort"
	"strings"
	"testing"
)

type coverageRow struct {
	name            string
	clean, preserve int
}

func bucket(n int) string {
	switch {
	case n == 0:
		return "0"
	case n == 1:
		return "1"
	}
	return "2+"
}

// singleSample reports whether one of the two kinds has exactly one fixture.
func (r coverageRow) singleSample() bool { return r.clean == 1 || r.preserve == 1 }

// manifestCounts counts clean and preserve fixtures per MANIFEST format key.
func manifestCounts(t *testing.T) map[string]*coverageRow {
	t.Helper()
	out := map[string]*coverageRow{}
	for _, e := range loadManifest(t) {
		row := out[string(e.format)]
		if row == nil {
			row = &coverageRow{name: string(e.format)}
			out[string(e.format)] = row
		}
		if e.kind == kindClean {
			row.clean++
		} else {
			row.preserve++
		}
	}
	return out
}

// formatCoverage has one row per Format constant, sorted by value.
func formatCoverage(t *testing.T) []coverageRow {
	t.Helper()
	counts := manifestCounts(t)
	seen := map[string]bool{}
	var rows []coverageRow
	for _, v := range formatConsts(t) {
		if seen[v] {
			continue
		}
		seen[v] = true
		row := coverageRow{name: v}
		if c := counts[v]; c != nil {
			row.clean, row.preserve = c.clean, c.preserve
		}
		rows = append(rows, row)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].name < rows[j].name })
	return rows
}

// familyCoverage has one row per execwrap Family. A compacting family sums the
// fixtures of the formats it may be compacted as; a pass-through family has
// only the preserve fixtures keyed by its own name.
func familyCoverage(t *testing.T) []coverageRow {
	t.Helper()
	counts, formats := manifestCounts(t), expectedFormatsOf(t)
	seen := map[string]bool{}
	var rows []coverageRow
	for _, fam := range familyConsts(t) {
		if seen[fam] {
			continue
		}
		seen[fam] = true
		row := coverageRow{name: fam}
		keys := formats[fam]
		if len(keys) == 0 {
			keys = []string{fam}
		}
		for _, k := range keys {
			if c := counts[k]; c != nil {
				row.clean += c.clean
				row.preserve += c.preserve
			}
		}
		rows = append(rows, row)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].name < rows[j].name })
	return rows
}

func renderCoverage(title string, rows []coverageRow) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s (%d)\n%-26s %-6s %-9s %s\n", title, len(rows), "name", "clean", "preserve", "fixtures")
	for _, r := range rows {
		fmt.Fprintf(&b, "%-26s %-6s %-9s %d clean, %d preserve\n", r.name, bucket(r.clean), bucket(r.preserve), r.clean, r.preserve)
	}
	return b.String()
}

func TestFixtureCoverageListsEveryFormatAndFamily(t *testing.T) {
	formats, families := formatCoverage(t), familyCoverage(t)
	report := renderCoverage("compressor formats", formats) + "\n" + renderCoverage("execwrap families", families)
	t.Log("\n" + report)

	for _, v := range formatConsts(t) {
		if !strings.Contains(report, "\n"+fmt.Sprintf("%-26s ", v)) {
			t.Errorf("format %q missing from the coverage report", v)
		}
	}
	for _, v := range familyConsts(t) {
		if !strings.Contains(report, "\n"+fmt.Sprintf("%-26s ", v)) {
			t.Errorf("family %q missing from the coverage report", v)
		}
	}
	if report != renderCoverage("compressor formats", formatCoverage(t))+"\n"+renderCoverage("execwrap families", familyCoverage(t)) {
		t.Error("coverage report is not deterministic")
	}
}

// TestFixtureCoverageSingleSample logs the families with exactly one clean or
// one preserve fixture: the candidates for a second fixture (BN-7).
func TestFixtureCoverageSingleSample(t *testing.T) {
	var names []string
	for _, r := range formatCoverage(t) {
		if r.singleSample() {
			names = append(names, fmt.Sprintf("%s(%dc/%dp)", r.name, r.clean, r.preserve))
		}
	}
	t.Logf("formats with a single-fixture side (%d): %s", len(names), strings.Join(names, " "))
}
