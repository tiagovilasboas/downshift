// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package execwrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// corpusFixture reads one file of the compressor's conformance corpus, so the
// wrapper and the compressor are held to the same real-shaped transcripts.
func corpusFixture(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "compressor", "testdata", "corpus", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// A failing yarn test run that cannot be cut passes through whole: every failure line is
// printed, on the stream the program wrote it to, with the program's own exit
// code.
func TestRun_FailingRunsOfPassThroughFamiliesStayWhole(t *testing.T) {
	cases := []struct {
		bin     string
		argv    []string
		fixture string
		failure []string
	}{
		{"yarn", []string{"yarn", "test"}, "yarn_test_fail.txt", []string{
			"FAIL src/utils/money.test.js",
			"  ● formatMoney › rounds half up",
			"error Command failed with exit code 1.",
		}},
	}
	for _, tc := range cases {
		t.Run(tc.fixture, func(t *testing.T) {
			raw := corpusFixture(t, tc.fixture)
			fakeBin(t, tc.bin, "cat <<'EOF'\n"+raw+"EOF\nexit 1\n")
			r, out, errOut := run(t, tc.argv, true)
			if r.ExitCode != 1 || r.Compacted {
				t.Fatalf("result=%+v", r)
			}
			if out != raw || errOut != "" {
				t.Fatalf("a failing run must pass through byte for byte:\nstdout=%q\nstderr=%q", out, errOut)
			}
			for _, l := range tc.failure {
				if !strings.Contains(out, l+"\n") {
					t.Errorf("lost failure line %q", l)
				}
			}
		})
	}
}

// pnpm test is compacted now. This small failing run stays whole only because
// compaction gains nothing on it, so it is asserted like the other failing
// runs.
func TestRun_SmallFailingPnpmRunStaysWhole(t *testing.T) {
	raw := corpusFixture(t, "pnpm_test_fail.txt")
	fakeBin(t, "pnpm", "cat <<'EOF'\n"+raw+"EOF\nexit 1\n")
	r, out, errOut := run(t, []string{"pnpm", "test"}, true)
	if r.ExitCode != 1 || r.Compacted {
		t.Fatalf("result=%+v", r)
	}
	if out != raw || errOut != "" {
		t.Fatalf("a failing run must pass through byte for byte:\nstdout=%q\nstderr=%q", out, errOut)
	}
	for _, l := range []string{
		" FAIL  src/lib/price.test.ts > applies the coupon discount",
		"AssertionError: expected 90 to be 81 // Object.is equality",
		" ELIFECYCLE  Test failed. See above for more details.",
	} {
		if !strings.Contains(out, l+"\n") {
			t.Errorf("lost failure line %q", l)
		}
	}
}

// go test writes module downloads and other notes on stderr while a clean
// run goes on. The run still compacts and every note is kept; a danger line
// or a failure on stderr keeps the whole run raw, each stream as written.
func TestRun_GoTestStderrNotes(t *testing.T) {
	suite := strings.ReplaceAll(passingSuite, "\t", "\\t")
	t.Run("a module download note compacts and stays", func(t *testing.T) {
		fakeBin(t, "go", "echo 'go: downloading golang.org/x/tools v0.1.0' >&2; printf '"+suite+"'")
		r, out, errOut := run(t, []string{"go", "test", "./..."}, true)
		if !r.Compacted || r.ExitCode != 0 || errOut != "" {
			t.Fatalf("result=%+v stderr=%q", r, errOut)
		}
		if !strings.Contains(out, "go: downloading golang.org/x/tools v0.1.0\n") || !strings.HasPrefix(out, "Go test: PASS (10 packages ok)") {
			t.Fatalf("stdout=%q", out)
		}
	})
	t.Run("a linker warning keeps the run raw", func(t *testing.T) {
		note := "# github.com/x/internal/a.test\nld: warning: ignoring duplicate libraries: -lm\n"
		fakeBin(t, "go", "printf '"+suite+"'; printf '"+strings.ReplaceAll(note, "\n", "\\n")+"' >&2")
		r, out, errOut := run(t, []string{"go", "test", "./..."}, true)
		if r.Compacted || out != passingSuite || errOut != note {
			t.Fatalf("result=%+v stdout=%q stderr=%q", r, out, errOut)
		}
	})
	t.Run("a panic on stderr with exit 0 keeps the run raw", func(t *testing.T) {
		fakeBin(t, "go", "printf '"+suite+"'; echo 'panic: boom' >&2")
		r, out, errOut := run(t, []string{"go", "test", "./..."}, true)
		if r.Compacted || out != passingSuite || errOut != "panic: boom\n" {
			t.Fatalf("result=%+v stdout=%q stderr=%q", r, out, errOut)
		}
	})
}

// A failed make target cuts only the passing go tests of its transcript; the
// failing target line, the failure and the make error line stay byte for byte.
// When make wrote to stderr the run is released as written (make is not held
// for its stderr), so this holds for a program that reports on stdout only.
func TestRun_FailedMakeCutsPassingGoTestsOnly(t *testing.T) {
	raw := corpusFixture(t, "make_check_fail.txt")
	fakeBin(t, "make", "cat <<'EOF'\n"+raw+"EOF\nexit 1\n")
	r, out, errOut := run(t, []string{"make", "check"}, true)
	if r.ExitCode != 1 || !r.Compacted || errOut != "" {
		t.Fatalf("result=%+v stderr=%q", r, errOut)
	}
	for _, l := range []string{
		"go vet ./...",
		"--- FAIL: TestCharge (0.00s)",
		"    billing_test.go:42: got 100, want 110",
		"FAIL\tgithub.com/acme/widget/internal/billing\t0.087s",
		"make: *** [Makefile:21: check] Error 1",
	} {
		if !strings.Contains(out, l+"\n") {
			t.Errorf("lost failure line %q:\n%s", l, out)
		}
	}
	if strings.Contains(out, "ok  \tgithub.com/acme/widget/internal/api") {
		t.Errorf("passing package lines must be cut:\n%s", out)
	}
	t.Run("stderr releases the run whole", func(t *testing.T) {
		fakeBin(t, "make", "cat <<'EOF'\n"+raw+"EOF\necho 'make: *** [Makefile:21: check] Error 1' >&2\nexit 1\n")
		r, out, errOut := run(t, []string{"make", "check"}, true)
		if r.ExitCode != 1 || r.Compacted || out != raw || errOut == "" {
			t.Fatalf("result=%+v stderr=%q", r, errOut)
		}
	})
}
