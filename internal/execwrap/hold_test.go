// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package execwrap

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

// Stdout is not held past HoldBound: a slow program shows its first output
// while it is still running, so a wrapper killed later has shown it, and the
// run is never compacted.
func TestRun_HoldIsBounded(t *testing.T) {
	old := HoldBound
	HoldBound = 100 * time.Millisecond
	t.Cleanup(func() { HoldBound = old })

	var suite strings.Builder
	for i := 0; i < 8; i++ {
		suite.WriteString("ok  \\tgithub.com/x/p\\t0.1s\\n")
	}
	fakeBin(t, "go", "printf '"+suite.String()+"'; sleep 1; printf 'ok  \\tgithub.com/x/q\\t0.1s\\n'")
	var merged lockedBuf
	seen := make(chan string, 1)
	go func() {
		var early string
		for end := time.Now().Add(2 * time.Second); time.Now().Before(end); time.Sleep(25 * time.Millisecond) {
			merged.mu.Lock()
			early = merged.b.String()
			merged.mu.Unlock()
			if strings.Contains(early, "github.com/x/p") {
				break
			}
		}
		seen <- early
	}()
	r := Run([]string{"go", "test"}, Options{Stdin: strings.NewReader(""), Stdout: &merged, Stderr: &merged, Compact: true})
	if early := <-seen; !strings.Contains(early, "github.com/x/p") {
		t.Fatalf("nothing shown while the program was still running: %q", early)
	}
	if r.Compacted || strings.Contains(merged.b.String(), "downshift exec:") {
		t.Fatalf("a run past the hold bound must stay raw: %+v %q", r, merged.b.String())
	}
	if r.RawBytes != r.EmittedBytes {
		t.Fatalf("raw=%d emitted=%d", r.RawBytes, r.EmittedBytes)
	}
}

// By default (HoldBound = 0), batch commands like go test hold the buffer
// until process completion (without short timeout) and compact upon exit 0.
func TestRun_BatchCommandHoldsBufferUntilExit(t *testing.T) {
	if HoldBound != 0 {
		t.Fatalf("expected default HoldBound == 0, got %v", HoldBound)
	}

	var suite strings.Builder
	for _, p := range []string{"a", "b", "c", "d", "e", "f", "g", "h"} {
		suite.WriteString("ok  \\tgithub.com/x/pkg/" + p + "\\t0.1s\\n")
	}
	suite.WriteString("?   \\tgithub.com/x/pkg/notest1\\t[no test files]\\n")
	suite.WriteString("?   \\tgithub.com/x/pkg/notest2\\t[no test files]\\n")

	// Sleep 200ms before finishing exit 0 to prove it is not discarded by any short timer.
	fakeBin(t, "go", "sleep 0.2; printf '"+suite.String()+"'")
	var out, errb bytes.Buffer
	r := Run([]string{"go", "test", "./..."}, Options{Stdin: strings.NewReader(""), Stdout: &out, Stderr: &errb, Compact: true})
	if r.ExitCode != 0 || r.ExitClass != ExitClassZero {
		t.Fatalf("exit = %d/%s, want 0/zero", r.ExitCode, r.ExitClass)
	}
	if !r.Compacted {
		t.Fatalf("expected batch command to compact upon exit 0: %+v\nout:\n%s", r, out.String())
	}
	if !strings.Contains(out.String(), "8 packages ok, 2 without tests") {
		t.Fatalf("expected summary to count ok and no-test packages: %s", out.String())
	}
	if !strings.Contains(out.String(), "[downshift exec: compacted") {
		t.Fatalf("expected compaction marker: %s", out.String())
	}
	if errb.Len() != 0 {
		t.Fatalf("expected empty stderr, got %q", errb.String())
	}
}

// Batch command with non-zero exit code must stay byte-identical raw.
func TestRun_BatchCommandNonZeroExitStaysRaw(t *testing.T) {
	var suite strings.Builder
	for _, p := range []string{"a", "b", "c", "d", "e", "f", "g", "h"} {
		suite.WriteString("ok  \tgithub.com/x/pkg/" + p + "\t0.1s\n")
	}
	suite.WriteString("?   \tgithub.com/x/pkg/notest\t[no test files]\n")
	rawWant := suite.String()

	fakeBin(t, "go", "printf '"+strings.ReplaceAll(rawWant, "\t", "\\t")+"'; exit 1")
	var out, errb bytes.Buffer
	r := Run([]string{"go", "test", "./..."}, Options{Stdin: strings.NewReader(""), Stdout: &out, Stderr: &errb, Compact: true})
	if r.ExitCode != 1 || r.ExitClass != ExitClassNonZero {
		t.Fatalf("exit = %d/%s, want 1/nonzero", r.ExitCode, r.ExitClass)
	}
	if r.Compacted {
		t.Fatalf("nonzero exit must not be compacted: %+v", r)
	}
	if out.String() != rawWant {
		t.Fatalf("stdout = %q, want raw %q", out.String(), rawWant)
	}
	if r.RawBytes != r.EmittedBytes {
		t.Fatalf("raw=%d emitted=%d", r.RawBytes, r.EmittedBytes)
	}
}
