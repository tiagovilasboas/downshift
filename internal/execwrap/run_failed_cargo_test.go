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

func cargoCorpus(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "compressor", "testdata", "corpus", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// cargo test writes its build status (Compiling, Finished, Running) on stderr
// and the test report on stdout. The wrapper merges both in write order, cuts
// the passing noise of a failed run, and returns the program's exit code.
func TestRun_FailedCargoTestCompactsMergedStreams(t *testing.T) {
	raw := cargoCorpus(t, "cargo_failed_lib_only.txt")
	body := compressor.CompressFailed([]byte(raw), compressor.FormatCargoTest).Output
	want := string(body) + Marker(len(raw), len(body))

	status, report, ok := strings.Cut(raw, "\nrunning 24 tests")
	if !ok {
		t.Fatal("fixture has no test report")
	}
	report = "\nrunning 24 tests" + report
	fakeBin(t, "cargo", emit(status, true)+gap+emit(report, false)+"exit 101")
	r, out, errOut := run(t, []string{"cargo", "test"}, true)
	if r.ExitCode != 101 || r.ExitClass != ExitClassNonZero || !r.Compacted || errOut != "" || out != want {
		t.Fatalf("result=%+v stdout=%q stderr=%q", r, out, errOut)
	}
	for _, l := range []string{"test billing::tests::invoice_total_with_discount ... FAILED\n", "  left: 9000\n", " right: 9500\n", "error: test failed, to rerun pass `--lib`\n"} {
		if !strings.Contains(out, l) {
			t.Errorf("lost failure line %q", l)
		}
	}
	if strings.Contains(out, "rejects_expired_token ... ok") {
		t.Errorf("passing test line kept")
	}
}

func TestRun_FailedCargoTestKillSwitchAndCompileErrorStayRaw(t *testing.T) {
	raw := cargoCorpus(t, "cargo_failed_lib_only.txt")
	fakeBin(t, "cargo", emit(raw, false)+"exit 101")
	if r, out, _ := run(t, []string{"cargo", "test"}, false); r.Compacted || out != raw || r.ExitCode != 101 {
		t.Fatalf("kill switch: compacted=%v code=%d", r.Compacted, r.ExitCode)
	}
	compileError := "   Compiling x v0.1.0 (/p)\nerror[E0425]: cannot find value `y` in this scope\n\nerror: could not compile `x` (lib test) due to 1 previous error\n"
	fakeBin(t, "cargo", emit(compileError, true)+"exit 101")
	if r, _, errOut := run(t, []string{"cargo", "test"}, true); r.Compacted || errOut != compileError || r.ExitCode != 101 {
		t.Fatalf("compile error: compacted=%v code=%d stderr=%q", r.Compacted, r.ExitCode, errOut)
	}
}
