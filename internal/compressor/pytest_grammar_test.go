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

const (
	pgHeader  = "============================= test session starts ==============================\n"
	pgPlatfV  = "platform linux -- Python 3.13.18, pytest-9.1.1, pluggy-1.6.0 -- /usr/bin/python\n"
	pgPlatf   = "platform linux -- Python 3.13.18, pytest-9.1.1, pluggy-1.6.0\n"
	pgSummary = "============================== %d passed in 0.09s ==============================\n"
)

// pgDots returns the wrapped progress lines of n passed tests the way pytest
// prints them at 80 columns: the file name opens the first line only.
func pgDots(n int) string {
	var b strings.Builder
	for i, first := 0, true; i < n; first = false {
		w := 60
		if n-i < w {
			w = n - i
		}
		if first {
			b.WriteString("test_ok.py ")
		}
		b.WriteString(strings.Repeat(".", w))
		i += w
		fmt.Fprintf(&b, " [%3d%%]\n", 100*i/n)
	}
	return b.String()
}

// pgVerbose returns n -v lines of passed tests whose ids need the wide node id
// shape: a space, brackets, angle brackets and an empty id.
func pgVerbose(n int) string {
	ids := []string{"test_a[a b]", "test_a[<lambda>]", "test_a[]", "TestK::test_m", "test_a[1.5-None]"}
	var b strings.Builder
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, "tests/test_a.py::%s PASSED%s[%3d%%]\n", ids[i%len(ids)], strings.Repeat(" ", 20), 100*(i+1)/n)
	}
	return b.String()
}

func TestPytestPassNewShapesCompact(t *testing.T) {
	cases := []struct {
		name, in string
		want     string
	}{
		{"progress line wrapped at the terminal width",
			pgHeader + pgPlatf + "rootdir: /work/proj\ncollected 120 items\n\n" + pgDots(120) + "\n" + fmt.Sprintf(pgSummary, 120),
			"Pytest: PASS (120 passed in 0.09s)\n"},
		{"-v run, the way pytest prints it",
			pgHeader + pgPlatfV + "cachedir: .pytest_cache\nrootdir: /work/proj\ncollecting ... collected 25 items\n\n" + pgVerbose(25) + "\n" + fmt.Sprintf(pgSummary, 25),
			"Pytest: PASS (25 passed in 0.09s)\n"},
		{"header with configfile and testpaths",
			pgHeader + pgPlatf + "rootdir: /work/proj\nconfigfile: pytest.ini\ntestpaths: tests\ncollected 40 items\n\n" + pgDots(40) + "\n" + fmt.Sprintf(pgSummary, 40),
			"Pytest: PASS (40 passed in 0.09s)\n"},
		{"several testpaths, one with a separator",
			pgHeader + pgPlatf + "rootdir: /work/proj\ntestpaths: tests src/unit\ncollected 40 items\n\n" + pgDots(40) + "\n" + fmt.Sprintf(pgSummary, 40),
			"Pytest: PASS (40 passed in 0.09s)\n"},
		{"passed and skipped dots on a wrapped line",
			pgHeader + pgPlatf + "rootdir: /work/proj\ncollected 62 items\n\ntest_ok.py " + strings.Repeat(".", 60) + "ss [100%]\n\n" +
				"======================== 60 passed, 2 skipped in 0.09s =========================\n",
			"Pytest: PASS (60 passed, 2 skipped in 0.09s)\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := compressor.Compress([]byte(tc.in), compressor.ModeSafe)
			if !res.Applied {
				t.Fatalf("not compacted: %q\n%s", res.Reason, tc.in)
			}
			if string(res.Output) != tc.want {
				t.Fatalf("got %q want %q", res.Output, tc.want)
			}
		})
	}
}

// A progress line or a -v line the reader dropped must show as a count that
// does not add up, and the run must stay raw.
func TestPytestPassCountsMustAddUp(t *testing.T) {
	head := pgHeader + pgPlatf + "rootdir: /work/proj\ncollected 10 items\n\n"
	cases := []struct{ name, in string }{
		{"fewer dots than passed", head + "test_ok.py ........ [100%]\n\n" + fmt.Sprintf(pgSummary, 10)},
		{"more dots than passed", head + "test_ok.py ............ [100%]\n\n" + fmt.Sprintf(pgSummary, 10)},
		{"a wrapped line lost", head + "test_ok.py .....\n\n" + fmt.Sprintf(pgSummary, 10)},
		{"fewer -v lines than passed", pgHeader + pgPlatfV + "rootdir: /work/proj\ncollecting ... collected 10 items\n\n" + pgVerbose(9) + "\n" + fmt.Sprintf(pgSummary, 10)},
		{"skipped counted but no s", head + "test_ok.py .......... [100%]\n\n======================== 10 passed, 2 skipped in 0.09s =========================\n"},
		{"s with no skipped counted", head + "test_ok.py .........s [100%]\n\n" + fmt.Sprintf(pgSummary, 10)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := []byte(tc.in)
			res := compressor.Compress(in, compressor.ModeSafe)
			if res.Applied || !bytes.Equal(res.Output, in) {
				t.Fatalf("a count that does not add up must keep the run raw: applied=%v reason=%q", res.Applied, res.Reason)
			}
		})
	}
}

// The wider shapes stay closed: anything that carries more than a passed test
// keeps the whole transcript raw.
func TestPytestPassNewShapesStayClosed(t *testing.T) {
	v := pgHeader + pgPlatfV + "rootdir: /work/proj\ncollecting ... collected 3 items\n\n"
	h := pgHeader + pgPlatf + "rootdir: /work/proj\n"
	three := fmt.Sprintf(pgSummary, 3)
	cases := []struct{ name, in string }{
		{"skipped with a reason", v + "tests/t.py::a PASSED   [ 33%]\ntests/t.py::b PASSED   [ 66%]\ntests/t.py::c SKIPPED (not now)   [100%]\n\n======================== 2 passed, 1 skipped in 0.09s =========================\n"},
		{"xfail line", v + "tests/t.py::a PASSED   [ 33%]\ntests/t.py::b PASSED   [ 66%]\ntests/t.py::c XFAIL   [100%]\n\n" + three},
		{"outcome word is not the last word", v + "tests/t.py::a PASSED   [ 33%]\ntests/t.py::b PASSED   [ 66%]\ntests/t.py::c PASSED extra text\n\n" + three},
		{"non-ASCII node id", v + "tests/t.py::a PASSED   [ 33%]\ntests/t.py::b PASSED   [ 66%]\ntests/t.py::\u00e9 PASSED   [100%]\n\n" + three},
		{"testpaths FAILED", h + "testpaths: FAILED\ncollected 3 items\n\ntest_ok.py ... [100%]\n\n" + three},
		{"testpaths with a danger word", h + "testpaths: tests error\ncollected 3 items\n\ntest_ok.py ... [100%]\n\n" + three},
		{"testpaths with a double space", h + "testpaths: tests  src\ncollected 3 items\n\ntest_ok.py ... [100%]\n\n" + three},
		{"empty testpaths", h + "testpaths: \ncollected 3 items\n\ntest_ok.py ... [100%]\n\n" + three},
		{"collecting line with trailing text", pgHeader + pgPlatfV + "collecting ... collected 3 items FAILED\n\ntests/t.py::a PASSED\ntests/t.py::b PASSED\ntests/t.py::c PASSED\n\n" + three},
		{"collecting line without a count", pgHeader + pgPlatfV + "collecting ... \n\ntests/t.py::a PASSED\ntests/t.py::b PASSED\ntests/t.py::c PASSED\n\n" + three},
		{"progress line with an F", h + "collected 3 items\n\ntest_ok.py ..F [100%]\n\n" + three},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := []byte(tc.in)
			res := compressor.Compress(in, compressor.ModeSafe)
			if res.Applied || !bytes.Equal(res.Output, in) {
				t.Fatalf("must stay raw: applied=%v reason=%q", res.Applied, res.Reason)
			}
		})
	}
}

// An unrecognised line injected into the new transcripts keeps them raw, at the
// head, the middle and the tail, as it does for the old ones.
func TestPytestPassInjectedLineKeepsNewShapesRaw(t *testing.T) {
	transcripts := map[string][]string{
		"wrapped": strings.Split(strings.TrimSuffix(pgHeader+pgPlatf+"rootdir: /work/proj\ncollected 120 items\n\n"+pgDots(120)+"\n"+fmt.Sprintf(pgSummary, 120), "\n"), "\n"),
		"verbose": strings.Split(strings.TrimSuffix(pgHeader+pgPlatfV+"rootdir: /work/proj\ncollecting ... collected 25 items\n\n"+pgVerbose(25)+"\n"+fmt.Sprintf(pgSummary, 25), "\n"), "\n"),
	}
	for name, clean := range transcripts {
		if res := compressor.Compress([]byte(strings.Join(clean, "\n")+"\n"), compressor.ModeSafe); !res.Applied {
			t.Fatalf("%s: clean transcript not compacted: %s", name, res.Reason)
		}
		for _, line := range []string{"unexpected error in output", "WARNING: something", "zz-unrecognised-line 42"} {
			for _, pos := range []int{0, len(clean) / 2, len(clean)} {
				lines := append(append(append([]string{}, clean[:pos]...), line), clean[pos:]...)
				in := []byte(strings.Join(lines, "\n") + "\n")
				t.Run(fmt.Sprintf("%s/%s@%d", name, line, pos), func(t *testing.T) {
					res := compressor.Compress(in, compressor.ModeSafe)
					if res.Applied || !bytes.Equal(res.Output, in) {
						t.Fatalf("compacted with an unrecognised line: %q", res.Reason)
					}
				})
			}
		}
	}
}

// A failing -v run prints "collecting ... collected N items", which used to keep
// the whole run raw. The passed lines are cut now and every failure line stays.
func TestCompressFailedPytestVerboseCollectingLine(t *testing.T) {
	failure := "=================================== FAILURES ===================================\n" +
		"________________________________ test_eq_wrong _________________________________\n\n" +
		"    def test_eq_wrong():\n>       assert 1 == 2\nE       assert 1 == 2\n\ntest_bad.py:9: AssertionError\n" +
		"=========================== short test summary info ============================\n" +
		"FAILED test_bad.py::test_eq_wrong - assert 1 == 2\n" +
		"========================= 1 failed, 40 passed in 0.04s =========================\n"
	in := []byte(pgHeader + pgPlatfV + "rootdir: /work/proj\ntestpaths: tests\ncollecting ... collected 41 items\n\n" +
		pgVerbose(40) + "tests/test_bad.py::test_eq_wrong FAILED   [100%]\n\n" + failure)
	r := compressor.CompressFailed(in, compressor.FormatPytest)
	if !r.Applied {
		t.Fatalf("failed -v run not compacted: %q", r.Reason)
	}
	out := string(r.Output)
	if !strings.HasPrefix(out, "Pytest: 40 passing tests cut (failures kept verbatim)\n") {
		t.Fatalf("summary line missing: %q", out)
	}
	for _, keep := range []string{"tests/test_bad.py::test_eq_wrong FAILED", "E       assert 1 == 2", "test_bad.py:9: AssertionError", "FAILED test_bad.py::test_eq_wrong - assert 1 == 2", "1 failed, 40 passed"} {
		if !strings.Contains(out, keep) {
			t.Fatalf("lost failure line %q:\n%s", keep, out)
		}
	}
	if strings.Contains(out, "PASSED") {
		t.Fatalf("a passed line survived:\n%s", out)
	}
}
