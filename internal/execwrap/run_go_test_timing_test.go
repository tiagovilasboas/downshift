// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package execwrap

import (
	"strings"
	"testing"
)

// TestRun_GoTestOverTwoSecondsCompacts verifies that commands taking longer than 2s
// (the previous HoldBound limit) are held until exit 0 and successfully compacted,
// rather than prematurely streaming raw due to time.
func TestRun_GoTestOverTwoSecondsCompacts(t *testing.T) {
	var suite strings.Builder
	for _, p := range []string{"a", "b", "c", "d", "e", "f", "g", "h", "i"} {
		suite.WriteString("ok  \tgithub.com/x/pkg/" + p + "\t0.1s\n")
	}
	suite.WriteString("?   \tgithub.com/x/pkg/notest1\t[no test files]\n")
	raw := suite.String()

	fakeBin(t, "go", "sleep 2.1; printf '"+strings.ReplaceAll(raw, "\t", "\\t")+"'")
	r, out, errb := run(t, []string{"go", "test", "./..."}, true)
	if r.ExitCode != 0 || r.ExitClass != ExitClassZero {
		t.Fatalf("exit = %d/%s, want 0/zero", r.ExitCode, r.ExitClass)
	}
	if !r.Compacted {
		t.Fatalf("command taking >2s was not compacted (prematurely aborted hold): %+v\nout:\n%s", r, out)
	}
	if !strings.Contains(out, "9 packages ok, 1 without tests") {
		t.Fatalf("expected summary in output: %q", out)
	}
	if !strings.Contains(out, "; original not kept]") {
		t.Fatalf("missing compaction marker in output: %q", out)
	}
	if errb != "" {
		t.Fatalf("expected empty stderr, got %q", errb)
	}
	if r.EmittedBytes >= r.RawBytes {
		t.Fatalf("emitted %d >= raw %d", r.EmittedBytes, r.RawBytes)
	}
}

// TestRun_GoTestOverTwoSecondsIncrementalCompacts proves that incremental stdout writes
// separated by more than 2 seconds are not prematurely flushed by the timer.
func TestRun_GoTestOverTwoSecondsIncrementalCompacts(t *testing.T) {
	part1 := "ok  \tgithub.com/x/pkg/a\t0.1s\nok  \tgithub.com/x/pkg/b\t0.1s\n"
	part2 := "ok  \tgithub.com/x/pkg/c\t0.1s\nok  \tgithub.com/x/pkg/d\t0.1s\nok  \tgithub.com/x/pkg/e\t0.1s\n?   \tgithub.com/x/pkg/notest\t[no test files]\n"

	// First chunk emitted immediately, second chunk emitted after 2.1s delay.
	// Under the old 2s HoldBound, part1 would have triggered the timer and flushed at 2s.
	fakeBin(t, "go", "printf '"+strings.ReplaceAll(part1, "\t", "\\t")+"'; sleep 2.1; printf '"+strings.ReplaceAll(part2, "\t", "\\t")+"'")
	r, out, errb := run(t, []string{"go", "test", "./..."}, true)
	if r.ExitCode != 0 || r.ExitClass != ExitClassZero {
		t.Fatalf("exit = %d/%s, want 0/zero", r.ExitCode, r.ExitClass)
	}
	if !r.Compacted {
		t.Fatalf("incremental write with >2s gap was not compacted: %+v\nout:\n%s", r, out)
	}
	if !strings.Contains(out, "5 packages ok, 1 without tests") {
		t.Fatalf("expected summary in output: %q", out)
	}
	if !strings.Contains(out, "; original not kept]") {
		t.Fatalf("expected compaction marker: %q", out)
	}
	if errb != "" {
		t.Fatalf("expected empty stderr, got %q", errb)
	}
}

// TestRun_GoTestOverTwoSecondsFailStaysRaw proves that a slow command (>2s)
// that fails (exit 1) remains 100% raw and is never compacted.
func TestRun_GoTestOverTwoSecondsFailStaysRaw(t *testing.T) {
	raw := "ok  \tgithub.com/x/pkg/a\t0.1s\n--- FAIL: TestFoo (0.01s)\nFAIL\nFAIL\tgithub.com/x/pkg/b\t0.02s\n?   \tgithub.com/x/pkg/c\t[no test files]\n"
	fakeBin(t, "go", "sleep 2.1; printf '"+strings.ReplaceAll(raw, "\t", "\\t")+"'; exit 1")
	r, out, _ := run(t, []string{"go", "test", "./..."}, true)
	if r.ExitCode != 1 || r.ExitClass != ExitClassNonZero {
		t.Fatalf("exit = %d/%s, want 1/nonzero", r.ExitCode, r.ExitClass)
	}
	if r.Compacted {
		t.Fatalf("failing command >2s must not be compacted: %+v", r)
	}
	if out != raw {
		t.Fatalf("output = %q, want raw %q", out, raw)
	}
	if r.RawBytes != r.EmittedBytes {
		t.Fatalf("raw %d != emitted %d", r.RawBytes, r.EmittedBytes)
	}
}
