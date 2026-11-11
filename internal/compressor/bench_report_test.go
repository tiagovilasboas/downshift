// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor_test

// Token-savings benchmark over the synthetic corpus. Report mode:
//
//	DOWNSHIFT_BENCH_WRITE=1 go test ./internal/compressor -run TestBenchReport
//
// (wrapped by `make bench`) rewrites docs/reference/bench.md and prints the
// table. Tokens are approximated as bytes/4. Nothing here runs a build or test
// tool: fixtures are replayed through the compressor. The per-family floor
// lives in bench_guard_test.go.

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

const benchDoc = "../../docs/reference/bench.md"

type benchRow struct {
	family  string
	files   int
	raw     int
	reduced int
	ccr     int // bytes after FoldRepeats on the compacted (or raw) output
}

func (r benchRow) saving() float64 { return savingPct(r.raw, r.reduced) }

func savingPct(raw, reduced int) float64 {
	if raw == 0 {
		return 0
	}
	return float64(raw-reduced) / float64(raw) * 100
}

// benchCorpus compacts every corpus fixture and aggregates the ones Downshift
// compacts, per family. The total row is last.
func benchCorpus(t *testing.T) []benchRow {
	t.Helper()
	by := map[string]*benchRow{}
	formats := map[string]compressor.Format{}
	for _, e := range loadManifest(t) {
		formats[e.file] = e.format
	}
	for _, name := range allFixtures(t) {
		if name == "MANIFEST" {
			continue
		}
		in, err := os.ReadFile(filepath.Join(corpusDir, name))
		if err != nil {
			t.Fatal(err)
		}
		res := benchCompress(in, formats[name])
		if !res.Applied || len(res.Output) >= len(in) {
			if f := compressor.FoldRepeats(in); f.Applied {
				row := by[ccrOnlyFamily]
				if row == nil {
					row = &benchRow{family: ccrOnlyFamily}
					by[ccrOnlyFamily] = row
				}
				row.files++
				row.raw += len(in)
				row.reduced += len(in)
				row.ccr += len(f.Output) + len(execwrap.MarkerWithID(len(in), len(f.Output), benchID))
			}
			continue
		}
		fam := string(res.Format)
		row := by[fam]
		if row == nil {
			row = &benchRow{family: fam}
			by[fam] = row
		}
		row.files++
		row.raw += len(in)
		row.reduced += len(res.Output) + len(execwrap.Marker(len(in), len(res.Output)))
		folded := foldedLen(res.Output)
		row.ccr += folded + len(execwrap.MarkerWithID(len(in), folded, benchID))
	}
	rows := make([]benchRow, 0, len(by)+1)
	total := benchRow{family: "TOTAL"}
	for _, r := range by {
		rows = append(rows, *r)
		if r.family == ccrOnlyFamily {
			continue // not counted in TOTAL: the default columns skip these fixtures
		}
		total.ccr += r.ccr
		total.files += r.files
		total.raw += r.raw
		total.reduced += r.reduced
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].family < rows[j].family })
	for i, r := range rows {
		if r.family == ccrOnlyFamily { // after the families, before TOTAL
			rows = append(append(rows[:i:i], rows[i+1:]...), r)
			break
		}
	}
	return append(rows, total)
}

// lossyFamilies lists the families whose default compaction drops content a
// reader could want: grouped or summarised diagnostics, cut or folded lines,
// stripped log noise. No property in internal/compressor marks this, so the
// classification is kept here beside the table; a family not listed is lossless
// (it keeps every failure and signal line, or replaces an all-pass transcript by
// its summary). The raw output of a lossy family is recoverable only through CCR.
var lossyFamilies = map[string]bool{
	"bazel": true, "cargo_build": true, "dotnet_build": true, "eslint": true,
	"generic": true, "gh_run_log": true, "git_log": true, "golangci_lint": true,
	"go_build": true, "gradle": true, "lockfile_diff": true, "markdown": true,
	"maven": true, "mypy": true, "npm_audit": true, "progress": true, "ruff": true,
	// Column tables cut to the columns an agent acts on; build steps folded;
	// stash entries without their branch.
	"docker_build": true, "docker_images": true, "docker_ps": true,
	"kubectl_pods": true, "git_stash": true,
	// A long source file folded to its outline; zero-count rows of grep -c folded.
	"file_read": true, "grep_counts": true,
	// A long response cut to its key headers and the ends of its body.
	"http_fetch": true,
	// Secrets replaced by [redacted]; nothing else cut.
	"glab":        true,
	ccrOnlyFamily: true,
}

func rowClass(family string) string {
	if family == "TOTAL" {
		return "-"
	}
	return familyClass(family)
}

func familyClass(family string) string {
	if lossyFamilies[family] {
		return "lossy"
	}
	return "lossless"
}

// classTotals sums the compacted families (not TOTAL, not the CCR-only row) by class.
func classTotals(rows []benchRow) (lossless, lossy benchRow) {
	lossless.family, lossy.family = "lossless", "lossy"
	for _, r := range rows {
		if r.family == "TOTAL" || r.family == ccrOnlyFamily {
			continue
		}
		dst := &lossless
		if lossyFamilies[r.family] {
			dst = &lossy
		}
		dst.files += r.files
		dst.raw += r.raw
		dst.reduced += r.reduced
	}
	return lossless, lossy
}

// ccrOnlyFamily groups fixtures the default chain leaves raw but FoldRepeats
// shortens; it has no default saving and is excluded from TOTAL.
const ccrOnlyFamily = "uncompacted (CCR only)"

// benchID stands in for the 12-hex retrieve id, so the CCR column carries the
// marker the compactor really emits.
const benchID = "000000000000"

// foldedLen is the size of out in the kept form: ShortenPass (a passing run's
// count line), then FoldRepeats; len(out) when neither applies.
func foldedLen(out []byte) int {
	out, _ = compressor.ShortenPass(out)
	if f := compressor.FoldRepeats(out); f.Applied {
		return len(f.Output)
	}
	return len(out)
}

func renderBench(rows []benchRow) string {
	var b strings.Builder
	b.WriteString("# Compaction benchmark\n\n")
	b.WriteString("Generated by `make bench` from `internal/compressor/testdata/corpus`; do not edit by hand.\n")
	b.WriteString("Only fixtures Downshift compacts (exit 0, safe mode) are counted; fixtures it preserves are excluded.\n")
	b.WriteString("Tokens are approximated as bytes/4, not a real tokenizer.\n")
	b.WriteString("Downshift and CCR bytes include the marker line the compactor appends at runtime; a family with one fixture is one sample, not a rate (see the Fixtures column).\n")
	b.WriteString("`with CCR` applies `FoldRepeats` (runs of 8+ similar lines folded to 3 head, 2 tail) to the compacted output, or to the raw output for the `uncompacted (CCR only)` row, which TOTAL excludes. It is lossy and is meant to run only when the raw output is stored for retrieval.\n\n")
	b.WriteString("| Family | Class | Fixtures | Raw bytes | Downshift bytes | Saving % | Raw tokens~ | Downshift tokens~ | with CCR bytes | with CCR saving % |\n")
	b.WriteString("|---|---|--:|--:|--:|--:|--:|--:|--:|--:|\n")
	for _, r := range rows {
		fmt.Fprintf(&b, "| %s | %s | %d | %d | %d | %.1f | %d | %d | %d | %.1f |\n", r.family, rowClass(r.family), r.files, r.raw, r.reduced, r.saving(), r.raw/4, r.reduced/4, r.ccr, savingPct(r.raw, r.ccr))
	}
	lossless, lossy := classTotals(rows)
	b.WriteString("\n## Lossless and lossy split\n\n")
	b.WriteString("`lossless` families keep every failure and signal line; `lossy` families cut, fold or summarise content, so the raw output is recoverable only through CCR. The split covers the compacted families; TOTAL is their sum.\n\n")
	b.WriteString("| Class | Fixtures | Raw bytes | Downshift bytes | Saving % |\n|---|--:|--:|--:|--:|\n")
	for _, r := range []benchRow{lossless, lossy} {
		fmt.Fprintf(&b, "| %s | %d | %d | %d | %.1f |\n", r.family, r.files, r.raw, r.reduced, r.saving())
	}
	b.WriteString("\n## Relationship to live token savings\n\n")
	total := rows[len(rows)-1]
	fmt.Fprintf(&b, "The figures above measure raw byte reduction across a synthetic corpus of %d compacted fixtures: command output and agent-desktop reads.\n\n", total.files)
	b.WriteString("- In live LLM sessions, compaction cuts verbose command outputs before they enter the prompt context, directly saving input tokens and subsequent prompt cache reads.\n")
	fmt.Fprintf(&b, "- With CCR enabled, overall corpus reduction is **%.1f%% fewer bytes** (and **%.1f%%** with CCR disabled).\n", savingPct(total.raw, total.ccr), total.saving())
	fmt.Fprintf(&b, "- Lossless families (%d fixtures) have **%.1f%% fewer bytes** while keeping every failure line, exit code and stack trace.\n", lossless.files, lossless.saving())
	b.WriteString("- Compaction operates on tool output bytes, whereas model routing operates on subagent model pricing. These levers are orthogonal and multiply when combined.\n")
	b.WriteString("- Detailed empirical reconciliation: [anthropic-references-benchmark-reconciliation-2026-11-07.md](../evidence/anthropic-references-benchmark-reconciliation-2026-11-07.md).\n")
	b.WriteString(balancedSection(balancedRows()))
	return b.String()
}

func TestBenchReport(t *testing.T) {
	rows := benchCorpus(t)
	doc := renderBench(rows)
	if os.Getenv("DOWNSHIFT_BENCH_WRITE") == "" {
		// Freshness mode: the committed report must equal what make bench renders.
		want, err := os.ReadFile(filepath.FromSlash(benchDoc))
		if err != nil {
			t.Fatal(err)
		}
		if string(want) != doc {
			t.Fatalf("docs/reference/bench.md is stale: it differs from the report the compressors produce now (first difference near byte %d); run `make bench` and commit the result", firstDiff(string(want), doc))
		}
		return
	}
	fmt.Print(doc)
	if err := os.WriteFile(filepath.FromSlash(benchDoc), []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}
}

// benchCompress is the compressor entry the wrapper uses for a fixture of
// format f. The formats a command names, not the text (file reads and the
// grep rows and counts), go through the command entry with the hint the
// conformance suite uses; every other fixture is detected from its text.
func benchCompress(in []byte, f compressor.Format) compressor.Result {
	switch f {
	case compressor.FormatFileRead, compressor.FormatGrepRows, compressor.FormatGrepCounts, compressor.FormatHTTPFetch, compressor.FormatGlab, compressor.FormatAgentDesktop:
		return compressFor(in, f, 0)
	}
	return compressor.CompressExit(in, compressor.ModeSafe, 0)
}

// firstDiff is the index of the first byte where a and b differ.
func firstDiff(a, b string) int {
	n := min(len(a), len(b))
	for i := 0; i < n; i++ {
		if a[i] != b[i] {
			return i
		}
	}
	return n
}
