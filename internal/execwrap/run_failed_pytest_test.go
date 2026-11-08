// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package execwrap

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/compressor"
)

// A failed pytest run prints its report on stdout. The wrapper compacts it,
// keeps every failure and returns the program's exit code. Any stderr byte
// releases the held output, so the run stays raw on its own streams. The run is
// the 25-test corpus fixture: a smaller one does not pay for its summary line.
func TestRun_FailedPytestCompactsOnlyWithoutStderr(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "compressor", "testdata", "corpus", "pytest_fail.txt"))
	if err != nil {
		t.Fatal(err)
	}
	body := compressor.CompressFailed(raw, compressor.FormatPytest).Output
	want := string(body) + Marker(len(raw), len(body))

	fakeBin(t, "pytest", emit(string(raw), false)+"exit 1")
	r, out, errOut := run(t, []string{"pytest"}, true)
	if r.ExitCode != 1 || r.ExitClass != ExitClassNonZero || !r.Compacted || errOut != "" || out != want {
		t.Fatalf("result=%+v stderr=%q out=%q", r, errOut, out)
	}

	fakeBin(t, "pytest", emit(string(raw), false)+"echo 'a warning' >&2\nexit 1")
	r, out, errOut = run(t, []string{"pytest"}, true)
	if r.Compacted || r.ExitCode != 1 || out != string(raw) || errOut != "a warning\n" {
		t.Fatalf("stderr: result=%+v", r)
	}
}
