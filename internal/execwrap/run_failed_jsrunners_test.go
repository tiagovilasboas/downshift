// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package execwrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/compressor"
)

// failedRunFixture reads one real failed-run transcript of the compressor.
func failedRunFixture(t *testing.T, dir, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "compressor", "testdata", dir, name+".in"))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// emit is a shell script that prints text exactly, on stdout or on stderr.
// gap separates two writes to different streams in a fake program. The wrapper
// reads stdout and stderr from two pipes with two goroutines, so two writes with
// no gap can reach it in either order on a loaded machine. A real runner's
// streams are not that tight, and the tests assume the order they were written in.
const gap = "sleep 0.05\n"

func emit(text string, toStderr bool) string {
	redirect := ""
	if toStderr {
		redirect = " >&2"
	}
	return "cat" + redirect + " <<'DOWNSHIFT_EOF'\n" + text + "DOWNSHIFT_EOF\n"
}

// A failed vitest run prints its report on stderr. The wrapper compacts the
// merged stream, keeps every failure, and returns the program's exit code.
func TestRun_FailedVitestCompactsAndKeepsExitCode(t *testing.T) {
	raw := failedRunFixture(t, "failed_vitest", "v4_fail")
	cut := strings.Index(raw, "⎯⎯⎯⎯⎯⎯⎯ Failed Tests")
	body := compressor.CompressFailed([]byte(raw), compressor.FormatVitest).Output
	want := string(body) + Marker(len(raw), len(body))

	t.Run("report on stderr, as vitest prints it", func(t *testing.T) {
		fakeBin(t, "vitest", emit(raw[:cut], false)+gap+emit(raw[cut:], true)+"exit 1")
		r, out, errOut := run(t, []string{"vitest", "run"}, true)
		if r.ExitCode != 1 || r.ExitClass != ExitClassNonZero || !r.Compacted || r.Passthrough != "" || errOut != "" {
			t.Fatalf("result=%+v stderr=%q", r, errOut)
		}
		if out != want {
			t.Fatalf("output\n got: %q\nwant: %q", out, want)
		}
		for _, l := range []string{" FAIL  test/billing.test.js > billing > applies the discount", "AssertionError: expected 100 to be 110 // Object.is equality", "Error: negative amount not allowed"} {
			if !strings.Contains(out, l+"\n") {
				t.Errorf("lost failure line %q", l)
			}
		}
	})
	t.Run("the whole report on stdout", func(t *testing.T) {
		fakeBin(t, "vitest", emit(raw, false)+"exit 1")
		if r, out, _ := run(t, []string{"vitest", "run"}, true); !r.Compacted || out != want {
			t.Fatalf("compacted=%v out=%q", r.Compacted, out)
		}
	})
	t.Run("kill switch keeps the run raw", func(t *testing.T) {
		fakeBin(t, "vitest", emit(raw[:cut], false)+gap+emit(raw[cut:], true)+"exit 1")
		r, out, errOut := run(t, []string{"vitest", "run"}, false)
		if r.Compacted || r.ExitCode != 1 || out != raw[:cut] || errOut != raw[cut:] {
			t.Fatalf("result=%+v", r)
		}
	})
	t.Run("a line the grammar does not know is kept, the run still compacts", func(t *testing.T) {
		odd := strings.Replace(raw, " ✓ test/ok.test.js", "something unexpected\n ✓ test/ok.test.js", 1)
		oddCut := strings.Index(odd, "⎯⎯⎯⎯⎯⎯⎯ Failed Tests")
		fakeBin(t, "vitest", emit(odd[:oddCut], false)+gap+emit(odd[oddCut:], true)+"exit 1")
		r, out, errOut := run(t, []string{"vitest", "run"}, true)
		if !r.Compacted || r.ExitCode != 1 || errOut != "" || !strings.Contains(out, "something unexpected\n") {
			t.Fatalf("result=%+v out=%q", r, out)
		}
	})
}

// A failed playwright run compacts when it wrote only to stdout. Any stderr
// byte releases the held output, so the run stays raw on its own streams.
func TestRun_FailedPlaywrightCompactsOnlyWithoutStderr(t *testing.T) {
	raw := failedRunFixture(t, "failed_playwright", "list_fail")
	body := compressor.CompressFailed([]byte(raw), compressor.FormatPlaywright).Output
	want := string(body) + Marker(len(raw), len(body))

	fakeBin(t, "playwright", emit(raw, false)+"exit 1")
	r, out, errOut := run(t, []string{"playwright", "test"}, true)
	if r.ExitCode != 1 || r.ExitClass != ExitClassNonZero || !r.Compacted || errOut != "" || out != want {
		t.Fatalf("result=%+v stderr=%q out=%q", r, errOut, out)
	}

	fakeBin(t, "playwright", emit(raw, false)+"echo 'a warning' >&2\nexit 1")
	r, out, errOut = run(t, []string{"playwright", "test"}, true)
	if r.Compacted || r.ExitCode != 1 || out != raw || errOut != "a warning\n" {
		t.Fatalf("stderr: result=%+v", r)
	}

	console := failedRunFixture(t, "failed_playwright", "list_fail_console_output")
	fakeBin(t, "playwright", emit(console, false)+"exit 1")
	r, out, _ = run(t, []string{"playwright", "test"}, true)
	if r.Compacted || out != console || r.Passthrough != PassthroughNonZeroExit {
		t.Fatalf("console output must keep the run raw: result=%+v", r)
	}
}
