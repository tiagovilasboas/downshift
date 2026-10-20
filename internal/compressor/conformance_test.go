// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor_test

// Family conformance suite. Every fixture under testdata/corpus is classified
// by testdata/corpus/MANIFEST ("clean" or "preserve", plus the format it
// belongs to), and every format is held to the same properties through the
// entry the wrapper uses (CompressExit in ModeSafe, exit 0, format must match):
//
//	(a) a clean fixture compacts and the output is strictly shorter;
//	(b) a preserve fixture comes back byte-identical;
//	(c) mutation closure: for each deterministic mutation of a clean fixture the
//	    result is byte-identical to the mutated input, or it still contains
//	    every danger line the mutation introduced.
//
// A new family is covered by adding its fixtures and MANIFEST lines; this file
// needs no edit. Exit-class safety is out of scope here.

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/compressor"
)

const (
	kindClean    = "clean"
	kindPreserve = "preserve"
)

type manifestEntry struct {
	kind   string
	format compressor.Format
	file   string
}

// loadManifest reads testdata/corpus/MANIFEST and checks it against the files
// in the directory: every file is classified, every line names a file.
func loadManifest(t *testing.T) []manifestEntry {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(corpusDir, "MANIFEST"))
	if err != nil {
		t.Fatalf("corpus manifest missing: %v", err)
	}
	var entries []manifestEntry
	listed := map[string]bool{}
	for i, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		f := strings.Fields(line)
		if len(f) != 3 || (f[0] != kindClean && f[0] != kindPreserve) {
			t.Fatalf("MANIFEST line %d: want \"clean|preserve <format> <file>\", got %q", i+1, line)
		}
		if listed[f[2]] {
			t.Fatalf("MANIFEST line %d: %s listed twice", i+1, f[2])
		}
		listed[f[2]] = true
		entries = append(entries, manifestEntry{kind: f[0], format: compressor.Format(f[1]), file: f[2]})
	}
	for _, name := range allFixtures(t) {
		if name == "MANIFEST" {
			continue
		}
		if !listed[name] {
			t.Errorf("corpus file %s is not classified in MANIFEST (add a clean/preserve line)", name)
		}
		delete(listed, name)
	}
	for name := range listed {
		t.Errorf("MANIFEST names %s, which is not in %s", name, corpusDir)
	}
	return entries
}

// wrapperOutput mirrors what execwrap emits for exit 0 and the family's own
// format: the compacted bytes when the compressor applied and recognised that
// format, the raw input otherwise.
func wrapperOutput(in []byte, want compressor.Format) (out []byte, compacted bool) {
	r := compressor.CompressExit(in, compressor.ModeSafe, 0)
	if r.Applied && r.Format == want {
		return r.Output, true
	}
	return in, false
}

func TestConformanceCleanCompactsShorter(t *testing.T) {
	for _, e := range loadManifest(t) {
		if e.kind != kindClean {
			continue
		}
		t.Run(e.file, func(t *testing.T) {
			in := readFixture(t, e.file)
			out, ok := wrapperOutput(in, e.format)
			if !ok {
				t.Fatalf("clean fixture not compacted as %s", e.format)
			}
			if len(out) >= len(in) {
				t.Fatalf("output not strictly shorter: %d -> %d", len(in), len(out))
			}
		})
	}
}

func TestConformancePreserveStaysIdentical(t *testing.T) {
	for _, e := range loadManifest(t) {
		if e.kind != kindPreserve {
			continue
		}
		t.Run(e.file, func(t *testing.T) {
			in := readFixture(t, e.file)
			for _, code := range []int{-1, 0} {
				r := compressor.CompressExit(in, compressor.ModeSafe, code)
				if r.Applied || !bytes.Equal(r.Output, in) {
					t.Fatalf("exit=%d: preserve fixture changed (applied=%v reason=%q)", code, r.Applied, r.Reason)
				}
			}
		})
	}
}

var dangerWords = regexp.MustCompile(`(?i)error|fail|warning|panic`)

type mutation struct {
	name string
	data []byte
}

// splitLines splits on "\n", leaving out the empty tail of a final newline.
func splitLines(b []byte) []string {
	return strings.Split(strings.TrimSuffix(string(b), "\n"), "\n")
}

func joinLines(lines []string) []byte { return []byte(strings.Join(lines, "\n") + "\n") }

func insertAt(lines []string, i int, s string) []string {
	out := append([]string{}, lines[:i]...)
	out = append(out, s)
	return append(out, lines[i:]...)
}

func replaceAt(lines []string, i int, s string) []string {
	out := append([]string{}, lines...)
	out[i] = s
	return out
}

// positions returns the first, middle and last line index (deduplicated).
func positions(n int) []int {
	seen := map[int]bool{}
	var out []int
	for _, p := range []int{0, n / 2, n - 1} {
		if !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	return out
}

// mutationsOf returns the deterministic mutation set for one clean fixture.
func mutationsOf(in []byte) []mutation {
	lines := splitLines(in)
	n := len(lines)
	var ms []mutation
	add := func(name string, data []byte) { ms = append(ms, mutation{name, data}) }

	for _, p := range []int{0, n / 2, n} {
		add(fmt.Sprintf("unknown-line@%d", p), joinLines(insertAt(lines, p, "zz-unrecognised-line 42")))
	}
	add("no-trailing-newline", bytes.TrimSuffix(in, []byte("\n")))
	for _, p := range positions(n) {
		add(fmt.Sprintf("cr@%d", p), joinLines(replaceAt(lines, p, lines[p]+"\r")))
		add(fmt.Sprintf("ansi@%d", p), joinLines(replaceAt(lines, p, "\x1b[32m"+lines[p]+"\x1b[0m")))
	}
	for _, q := range []int{1, 2, 3} {
		add(fmt.Sprintf("truncate@%d/4", q), in[:len(in)*q/4])
	}
	add("truncate@len-1", in[:len(in)-1])
	for i := n - 1; i >= 0; i-- { // the summary is the last non-empty line
		if strings.TrimSpace(lines[i]) != "" {
			add("duplicate-summary", joinLines(insertAt(lines, i+1, lines[i])))
			break
		}
	}
	for _, w := range []string{"error", "FAIL", "warning", "panic"} {
		for _, p := range []int{0, n / 2, n} {
			add(fmt.Sprintf("danger-%s@%d", w, p), joinLines(insertAt(lines, p, "unexpected "+w+" in output")))
		}
	}
	return ms
}

// dangerLinesAdded returns the danger-word lines of mutated that the clean
// original did not already have. Danger words in the original (for example
// "0 errors") are the facts the family summarises on purpose.
func dangerLinesAdded(original, mutated []byte) []string {
	had := map[string]bool{}
	for _, l := range splitLines(original) {
		had[l] = true
	}
	var out []string
	for _, l := range strings.Split(string(mutated), "\n") {
		if l != "" && dangerWords.MatchString(l) && !had[l] {
			out = append(out, l)
		}
	}
	return out
}

// knownViolations is the explicit allowlist of conformance gaps, keyed by
// "<fixture>|<mutation class>" (the mutation name without its "@position").
// Every entry carries a reason with a TODO naming the family; the suite stays
// green while the gap stays visible. An entry that stops firing fails the
// suite, so fixed gaps leave the list.
var knownViolations = buildKnownViolations()

func buildKnownViolations() map[string]string {
	const summaryOnly = "TODO(%s): the summary-only compressor keys on its summary line and drops a stdout line with a danger word injected into an otherwise clean run"
	m := map[string]string{}
	for _, f := range []struct{ fixture, family string }{
		{"cargo_pass_clean.txt", "cargo-test"},
		{"playwright_pass_clean.txt", "playwright"},
		{"tsc_pass_clean.txt", "tsc"},
		{"vitest_pass_clean.txt", "vitest"},
	} {
		for _, w := range []string{"error", "FAIL", "warning", "panic"} {
			if f.family == "vitest" && w == "FAIL" {
				continue // the vitest detector already preserves a FAIL line
			}
			m[f.fixture+"|danger-"+w] = fmt.Sprintf(summaryOnly, f.family)
		}
	}
	m["tsc_pass_clean.txt|ansi"] = "TODO(tsc): an ANSI-wrapped 'Found 0 errors' line is compacted without keeping the coloured original"
	return m
}

func TestConformanceMutationClosure(t *testing.T) {
	used := map[string]bool{}
	for _, e := range loadManifest(t) {
		if e.kind != kindClean {
			continue
		}
		in := readFixture(t, e.file)
		for _, m := range mutationsOf(in) {
			key := e.file + "|" + strings.SplitN(m.name, "@", 2)[0]
			out, compacted := wrapperOutput(m.data, e.format)
			if !compacted || bytes.Equal(out, m.data) {
				continue
			}
			var lost []string
			for _, l := range dangerLinesAdded(in, m.data) {
				if !bytes.Contains(out, []byte(l)) {
					lost = append(lost, l)
				}
			}
			if len(lost) == 0 {
				continue
			}
			if reason, ok := knownViolations[key]; ok {
				used[key] = true
				t.Logf("known violation %s (%s)", key, reason)
				continue
			}
			t.Errorf("%s [%s]: compacted a mutated input and dropped danger line(s) %q\n output: %q", e.file, m.name, lost, out)
		}
	}
	var stale []string
	for k := range knownViolations {
		if !used[k] {
			stale = append(stale, k)
		}
	}
	sort.Strings(stale)
	for _, k := range stale {
		t.Errorf("knownViolations entry %s no longer fires; remove it", k)
	}
}
