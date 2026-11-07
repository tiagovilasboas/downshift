// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor_test

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/compressor"
)

func TestCompress_Pytest_PassCompaction(t *testing.T) {
	in := []byte("============================= test session starts ==============================\nrootdir: /tmp\ncollected 10 items\n\ntest_a.py ..........\n\n============================== 10 passed in 0.42s ==============================\n")
	res := compressor.Compress(in, compressor.ModeSafe)
	if !res.Applied {
		t.Fatalf("expected applied=true for passing pytest: %s", res.Reason)
	}
	want := "Pytest: PASS (10 passed in 0.42s)\n"
	if string(res.Output) != want {
		t.Errorf("got %q, want %q", string(res.Output), want)
	}
	if res.Format != compressor.FormatPytest {
		t.Errorf("expected format pytest, got %v", res.Format)
	}
}

func TestCompress_Pytest_QuietPassCompaction(t *testing.T) {
	in := []byte("........................................                                 [100%]\n40 passed in 0.03s\n")
	res := compressor.Compress(in, compressor.ModeSafe)
	if !res.Applied {
		t.Fatalf("expected applied=true for passing pytest -q: %s", res.Reason)
	}
	want := "Pytest: PASS (40 passed in 0.03s)\n"
	if string(res.Output) != want {
		t.Errorf("got %q, want %q", string(res.Output), want)
	}
}

func TestCompress_Pytest_PassWithSkipped(t *testing.T) {
	in := []byte("============================= test session starts ==============================\ncollected 12 items\n\ntest_a.py ..........ss\n\n======================== 10 passed, 2 skipped in 1.15s =========================\n")
	res := compressor.Compress(in, compressor.ModeSafe)
	if !res.Applied {
		t.Fatalf("expected applied=true for pytest with skipped: %s", res.Reason)
	}
	want := "Pytest: PASS (10 passed, 2 skipped in 1.15s)\n"
	if string(res.Output) != want {
		t.Errorf("got %q, want %q", string(res.Output), want)
	}
}

func TestCompress_Pytest_ErrorsPreserveRaw(t *testing.T) {
	cases := []struct {
		name string
		in   string
	}{
		{"FAILED line", "=== 5 passed in 0.1s ===\nFAILED test_x.py::test_1\n"},
		{"ERROR line", "=== 5 passed in 0.1s ===\nERROR: fixture error\n"},
		{"Traceback line", "=== 5 passed in 0.1s ===\nTraceback (most recent call last):\n"},
		{"AssertionError line", "=== 5 passed in 0.1s ===\nAssertionError: assert 1 == 2\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := []byte(tc.in)
			res := compressor.Compress(in, compressor.ModeSafe)
			if res.Applied {
				t.Fatalf("must not compress pytest with error: %s", tc.name)
			}
			if !bytes.Equal(res.Output, in) {
				t.Fatalf("must preserve raw output byte-identical")
			}
		})
	}
}

// TestCompress_Pytest_UnrecognisedLinePreservesRaw injects lines that a clean
// run never emits (danger words and a plain unknown line) at the head, middle
// and tail of a passing transcript. Each must come back byte-identical: a
// failure or warning must never be hidden behind the one-line summary.
func TestCompress_Pytest_UnrecognisedLinePreservesRaw(t *testing.T) {
	clean := []string{
		"============================= test session starts ==============================",
		"platform darwin -- Python 3.12.3, pytest-8.1.1, pluggy-1.4.0",
		"rootdir: /Users/tiago.boas/project",
		"collected 10 items",
		"",
		"tests/test_a.py ..........                                             [100%]",
		"",
		"============================== 10 passed in 0.42s ==============================",
	}
	injected := []string{
		"unexpected error in output",
		"unexpected FAIL in output",
		"unexpected warning in output",
		"unexpected panic in output",
		"zz-unrecognised-line 42",
	}
	for _, line := range injected {
		for _, pos := range []int{0, len(clean) / 2, len(clean)} {
			lines := append(append(append([]string{}, clean[:pos]...), line), clean[pos:]...)
			in := []byte(strings.Join(lines, "\n") + "\n")
			t.Run(fmt.Sprintf("%s@%d", line, pos), func(t *testing.T) {
				res := compressor.Compress(in, compressor.ModeSafe)
				if res.Applied {
					t.Fatalf("compacted pytest with an unrecognised line: %q", res.Reason)
				}
				if !bytes.Equal(res.Output, in) {
					t.Fatalf("output not byte-identical to input:\n got %q\nwant %q", res.Output, in)
				}
			})
		}
	}
	// Without the injected line the same transcript still compacts.
	res := compressor.Compress([]byte(strings.Join(clean, "\n")+"\n"), compressor.ModeSafe)
	if !res.Applied {
		t.Fatalf("clean transcript not compacted: %s", res.Reason)
	}
}

// TestCompress_Pytest_ClosedGrammarPreservesRaw covers free text that a loose
// pattern used to accept: a header or plugin line carrying a failure, and a
// passing summary with trailing spaces. Each must come back byte-identical.
func TestCompress_Pytest_ClosedGrammarPreservesRaw(t *testing.T) {
	const header = "============================= test session starts ==============================\n"
	const summary = "============================== 10 passed in 0.42s ==============================\n"
	cases := []struct {
		name string
		in   string
	}{
		{"plugins line carries FAILED text", header + "plugins: FAILED tests/test_a.py::t - AssertionError\n" + summary},
		{"rootdir with spaces", header + "rootdir: /tmp/my project\n" + summary},
		{"rootdir without a path", header + "rootdir: FAILED\n" + summary},
		{"summary with trailing space", header + "tests/test_a.py ..........\n=============================== 10 passed in 0.42s =============================== \n"},
		{"summary with trailing tab", header + "tests/test_a.py ..........\n=============================== 10 passed in 0.42s ===============================\t\n"},
		{"summary time with a failure suffix", header + "tests/test_a.py ..........\n=== 10 passed in 0.42s - 1 failed ===\n"},
		{"summary time not a number", header + "tests/test_a.py ..........\n=== 10 passed in FAILED ===\n"},
		{"configfile value FAILED", header + "configfile: FAILED\n" + summary},
		{"configfile value with a space", header + "configfile: pytest ini FAILED\n" + summary},
		{"cachedir value with a space", header + "cachedir: .pytest_cache FAILED\n" + summary},
		{"platform python version with text", header + "platform darwin -- Python 3.12.3 FAILED, pytest-8.1.1\n" + summary},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := []byte(tc.in)
			res := compressor.Compress(in, compressor.ModeSafe)
			if res.Applied {
				t.Fatalf("compacted pytest with free text: %q", res.Reason)
			}
			if !bytes.Equal(res.Output, in) {
				t.Fatalf("output not byte-identical to input:\n got %q\nwant %q", res.Output, in)
			}
		})
	}
}
