// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func passRun(t *testing.T, fixture string, family Family, exit int, keep func([]byte) (string, bool)) (held, []byte) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "compressor", "testdata", "corpus", fixture))
	if err != nil {
		t.Fatal(err)
	}
	argv := []string{"go", "test", "./..."}
	return compactHeld(argv, raw, family, exit, false, keep, nil, false, balanceConfig{}), raw
}

// With a stored copy a passing run keeps only the count line, and the marker
// names the id that brings the raw output back. Without one, the rows stay.
func TestKeptPassSummary_CopyKeptOrNot(t *testing.T) {
	var calls [][]byte
	withCopy, raw := passRun(t, "gotest_pass_coverage.txt", FamilyGoTest, 0, storeOK(&calls))
	if len(calls) != 1 || string(calls[0]) != string(raw) {
		t.Fatalf("the raw output was not offered to keep exactly once: %d calls", len(calls))
	}
	out := string(withCopy.out)
	if !strings.HasPrefix(out, "Go test: PASS (12 packages ok, coverage min 59.2% max 100.0%)\n") ||
		strings.Contains(out, "\nok ") || !strings.Contains(out, "downshift retrieve 0123456789ab") {
		t.Fatalf("kept form: %q", out)
	}

	for name, keep := range map[string]func([]byte) (string, bool){
		"no keep port":  nil,
		"store refused": func([]byte) (string, bool) { return "", false },
		"store panics":  func([]byte) (string, bool) { panic("disk") },
	} {
		t.Run(name, func(t *testing.T) {
			h, _ := passRun(t, "gotest_pass_coverage.txt", FamilyGoTest, 0, keep)
			got := string(h.out)
			if !strings.Contains(got, "ok cmd/widget 71.4%") || !strings.Contains(got, "ok internal/store 86.5%") ||
				!strings.Contains(got, "original not kept") {
				t.Fatalf("a run with no stored copy lost what today's output keeps: %q", got)
			}
		})
	}
}

// A failed run is not touched by the kept form.
func TestKeptPassSummary_FailedRunUnchanged(t *testing.T) {
	var calls [][]byte
	h, _ := passRun(t, "gotest_fail_mixed_ok.txt", FamilyGoTest, 1, storeOK(&calls))
	if strings.Contains(string(h.out), "PASS (") {
		t.Fatalf("a failing run got a pass summary: %q", h.out)
	}
}

// The kept form is never longer than the raw output, marker included.
func TestKeptPassSummary_NeverLongerThanRaw(t *testing.T) {
	for _, name := range []string{
		"gotest_pass_coverage.txt", "gotest_no_test_files.txt", "gotest_pass_mixed_no_test_files.txt",
		"gotest_pass_plain.txt", "gotest_pass_cached.txt", "gotest_ansi.txt", "pytest_pass_clean.txt",
	} {
		var calls [][]byte
		h, raw := passRun(t, name, FamilyGoTest, 0, storeOK(&calls))
		if h.compacted && len(h.out) >= len(raw) {
			t.Errorf("%s: %d bytes out, %d raw", name, len(h.out), len(raw))
		}
	}
	// Boundary: a raw output just long enough for the default summary but not
	// for the longest retrieve hint is not stored, and stays shorter than raw.
	raw := []byte("........................................                                 [100%]\n40 passed in 0.03s\n")
	for n := len(raw) - 12; n <= len(raw); n++ {
		var calls [][]byte
		r := raw[:0:0]
		r = append(r, raw...)
		r = append(r[:n-1], '\n')
		h := compactHeld([]string{"pytest"}, r, FamilyPytest, 0, false, storeOK(&calls), nil, false, balanceConfig{})
		if h.compacted && len(h.out) >= len(r) {
			t.Errorf("raw %d bytes: output %d bytes", len(r), len(h.out))
		}
	}
}
