// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor_test

// Lossless invariant (BN-4). A family that is not in lossyFamilies promises to
// keep every signal line of its input. For each such family with a clean
// MANIFEST fixture, the invariant in lossless_invariant_checks_test.go compares
// the wrapper output with the input. A lossless family with no invariant fails
// unless it is in invariantAllowlist with a reason.

import (
	"sort"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/compressor"
)

// losslessCleanFormats returns the formats that have a clean fixture and are
// not classified lossy.
func losslessCleanFormats(t *testing.T) []compressor.Format {
	t.Helper()
	seen := map[compressor.Format]bool{}
	var out []compressor.Format
	for _, e := range loadManifest(t) {
		if e.kind == kindClean && !lossyFamilies[string(e.format)] && !seen[e.format] {
			seen[e.format] = true
			out = append(out, e.format)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func TestLosslessEveryFamilyHasInvariantOrAllowlistEntry(t *testing.T) {
	lossless := map[compressor.Format]bool{}
	for _, f := range losslessCleanFormats(t) {
		lossless[f] = true
		_, hasInv := invariants[f]
		reason, listed := invariantAllowlist[f]
		switch {
		case !hasInv && !listed:
			t.Errorf("lossless family %q has no invariant: add one to invariants, or a reasoned entry to invariantAllowlist", f)
		case hasInv && listed:
			t.Errorf("family %q has an invariant and an allowlist entry; drop the entry (%s)", f, reason)
		case listed && strings.TrimSpace(reason) == "":
			t.Errorf("allowlist entry %q needs a reason", f)
		}
	}
	for f := range invariants {
		if !lossless[f] {
			t.Errorf("invariant for %q, which is not a lossless family with a clean fixture", f)
		}
	}
	for f := range invariantAllowlist {
		if !lossless[f] {
			t.Errorf("stale allowlist entry %q: not a lossless family with a clean fixture", f)
		}
	}
}

func TestLosslessSignalLinesSurvive(t *testing.T) {
	for _, e := range loadManifest(t) {
		inv, ok := invariants[e.format]
		if e.kind != kindClean || !ok {
			continue
		}
		t.Run(e.file, func(t *testing.T) {
			in := readFixture(t, e.file)
			out, compacted := wrapperOutput(in, e.format)
			if !compacted {
				t.Fatalf("clean fixture not compacted as %s", e.format)
			}
			if lost := inv(in, out); len(lost) > 0 {
				t.Errorf("%s lost signal: %q\n output: %q", e.format, lost, out)
			}
		})
	}
}

// TestLosslessInvariantBites proves the helpers fail on a lossy output: each
// case feeds an invariant a compaction that dropped one signal.
func TestLosslessInvariantBites(t *testing.T) {
	grepIn := []byte("a/b.go:14:\tfoo()\na/b.go:31:\tbar()\nc/d.go:7:\tbaz()\n")
	cases := []struct {
		name    string
		inv     invariant
		in, out string
	}{
		{"grep match dropped", grepInvariant, string(grepIn), "a/b.go\n 14: foo()\nc/d.go\n 7: baz()\n"},
		{"grep file dropped", grepInvariant, string(grepIn), "a/b.go\n 14: foo()\n 31: bar()\n"},
		{"grep line number wrong", grepInvariant, string(grepIn), "a/b.go\n 14: foo()\n 32: bar()\nc/d.go\n 7: baz()\n"},
		{"failure line dropped", runnerInvariant(maxOf(rePassedN)),
			"FAIL: TestX\n24 passed, 1 failed\n", "Pytest: FAIL (24 passed)\n"},
		{"pass total changed", runnerInvariant(maxOf(rePassedN)),
			"===== 25 passed in 0.4s =====\n", "Pytest: PASS (24 passed)\n"},
		{"panic dropped", runnerInvariant(goTestTotal),
			"panic: boom\nok  \tpkg/a\t0.1s\n", "Go test: PASS (1 packages ok)\n"},
		{"package count wrong", runnerInvariant(goTestTotal),
			"ok  \tpkg/a\t0.1s\nok  \tpkg/b\t0.1s\n", "Go test: PASS (1 packages ok)\n"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if lost := c.inv([]byte(c.in), []byte(c.out)); len(lost) == 0 {
				t.Fatalf("invariant accepted a lossy output:\n in: %q\nout: %q", c.in, c.out)
			}
		})
	}
	t.Run("faithful output passes", func(t *testing.T) {
		if lost := grepInvariant(grepIn, []byte("a/b.go\n 14: foo()\n 31: bar()\nc/d.go\n 7: baz()\n")); len(lost) > 0 {
			t.Fatalf("faithful grouping flagged: %q", lost)
		}
	})
}

// TestLosslessInvariantBitesOnRealFixtures injects a failure line into each
// fixture of an invariant family and keeps the output unchanged: the line is
// then lost by construction, and the invariant must say so.
func TestLosslessInvariantBitesOnRealFixtures(t *testing.T) {
	for _, e := range loadManifest(t) {
		inv, ok := invariants[e.format]
		if e.kind != kindClean || !ok || e.format == compressor.FormatGrep || e.format == compressor.FormatGrepRows {
			continue
		}
		t.Run(e.file, func(t *testing.T) {
			in := readFixture(t, e.file)
			out, _ := wrapperOutput(in, e.format)
			mutated := append([]byte("FAIL injected-by-guard 0.1s\n"), in...)
			if lost := inv(mutated, out); len(lost) == 0 {
				t.Fatalf("invariant did not notice a dropped FAIL line for %s", e.format)
			}
		})
	}
}
