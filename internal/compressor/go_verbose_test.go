// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseGoVerboseCompactsPassingTests(t *testing.T) {
	in := readGoVerboseFixture(t, "go_verbose_pass.txt")
	out, applied, reason := parseGoVerbose(in)
	if !applied {
		t.Fatalf("expected compaction, reason=%q", reason)
	}
	for _, fact := range []string{"PASS", "3 passed", "0 failed", "internal/config"} {
		if !strings.Contains(string(out), fact) {
			t.Errorf("compacted output lost %q: %s", fact, out)
		}
	}
	if strings.Contains(string(out), "=== RUN") || len(out) >= len(in) {
		t.Fatalf("expected smaller summary without test runs, got %d bytes of %d: %s", len(out), len(in), out)
	}
}

func TestCompressGoVerbosePassesThroughPublicCompressor(t *testing.T) {
	in := readGoVerboseFixture(t, "go_verbose_pass.txt")
	res := CompressExit(in, ModeSafe, 0)
	if !res.Applied || res.Format != FormatGoTest {
		t.Fatalf("verbose pass was not compacted: applied=%v format=%q reason=%q", res.Applied, res.Format, res.Reason)
	}
}

func TestParseGoVerbosePreservesFailureOutput(t *testing.T) {
	in := readGoVerboseFixture(t, "go_verbose_fail.txt")
	out, applied, reason := parseGoVerbose(in)
	if applied || !bytes.Equal(out, in) {
		t.Fatalf("failing suite must be preserved byte-identically: applied=%v reason=%q", applied, reason)
	}
}

// A parent whose subtests all pass and print nothing compacts. Counts are the
// top-level tests; the subtests are reported separately and never inflate them.
func TestParseGoVerboseCompactsPassingSubtests(t *testing.T) {
	in := []byte("=== RUN   TestParent\n=== RUN   TestParent/a\n=== RUN   TestParent/b,_c\n=== RUN   TestParent/b,_c/deep\n" +
		"--- PASS: TestParent (0.00s)\n    --- PASS: TestParent/a (0.00s)\n    --- SKIP: TestParent/b,_c (0.00s)\n        --- PASS: TestParent/b,_c/deep (0.00s)\n" +
		"=== RUN   TestOther\n--- PASS: TestOther (0.00s)\nPASS\nok  \texample.test/pkg\t0.012s\n")
	out, applied, reason := parseGoVerbose(in)
	if !applied {
		t.Fatalf("passing subtests must compact, reason=%q", reason)
	}
	for _, fact := range []string{"PASS", "2 passed", "0 failed", "Subtests: 3", "example.test/pkg", "0.012s"} {
		if !strings.Contains(string(out), fact) {
			t.Errorf("compacted output lost %q: %s", fact, out)
		}
	}
}

func TestParseGoVerboseFallsBackForUnexpectedOrAmbiguousOutput(t *testing.T) {
	fixtures := [][]byte{
		[]byte("=== RUN   TestExample\n--- PASS: TestExample (0.00s)\nPASS\nok example.test/pkg 0.001s\nextra output\n"),
		[]byte("=== RUN   TestParent\n=== RUN   TestParent/child\n--- FAIL: TestParent (0.00s)\n    --- FAIL: TestParent/child (0.00s)\nFAIL\nFAIL\texample.test/pkg\t0.001s\n"),
		[]byte("=== RUN   TestParent\n=== RUN   TestParent/child\n    log line\n--- PASS: TestParent (0.00s)\n    --- PASS: TestParent/child (0.00s)\nPASS\nok example.test/pkg 0.001s\n"),
		[]byte("=== RUN   TestExample\n--- PASS: TestExample (0.00s)\nPASS\n"),
		[]byte("=== RUN   TestExample\n    useful output\n--- PASS: TestExample (0.00s)\nPASS\nok example.test/pkg 0.001s\n"),
	}
	for i, in := range fixtures {
		t.Run(string(rune('a'+i)), func(t *testing.T) {
			out, applied, _ := parseGoVerbose(in)
			if applied || !bytes.Equal(out, in) {
				t.Fatalf("unexpected output must fall back byte-identically: applied=%v output=%q", applied, out)
			}
		})
	}
}

func readGoVerboseFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "corpus", name))
	if err != nil {
		t.Fatalf("read Go verbose fixture %s: %v", name, err)
	}
	return b
}

// Real `go test -v` output of a package whose tests use subtests and print
// nothing (hookutil, durations normalised): Go prints each parent's outcome
// first and its indented subtest outcomes after it.
func TestParseGoVerboseCompactsRealSubtestTranscript(t *testing.T) {
	in := readGoVerboseFixture(t, "go_verbose_subtests_pass.txt")
	out, applied, reason := parseGoVerbose(in)
	if !applied {
		t.Fatalf("real subtest transcript must compact, reason=%q", reason)
	}
	for _, fact := range []string{"Go test -v: PASS", "Subtests: ", "hookutil"} {
		if !strings.Contains(string(out), fact) {
			t.Errorf("compacted output lost %q: %s", fact, out)
		}
	}
	if len(out)*4 > len(in) {
		t.Errorf("expected at least 4x smaller, got %d of %d bytes", len(out), len(in))
	}
}
