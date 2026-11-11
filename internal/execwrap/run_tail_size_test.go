// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package execwrap

import (
	"bytes"
	"strings"
	"testing"
)

const cleanStatus = "On branch main\nYour branch is up to date with origin/main.\n\nnothing to commit, working tree clean\n"

// The never-longer-than-raw guarantee counts the tail notes a compacted run
// appends: a summary that only fits without them stays raw, and EmittedBytes
// is what the wrapper really wrote.
func TestRunTailNotesCountInSizeGuarantee(t *testing.T) {
	for _, o := range []Options{{DiscardStderr: true}, {DiscardStderr: true, ExitZero: true}} {
		fakeBin(t, "git", shellPrintf(cleanStatus))
		var out bytes.Buffer
		o.Stdin, o.Stdout, o.Stderr, o.Compact = strings.NewReader(""), &out, &bytes.Buffer{}, true
		r := Run([]string{"git", "status"}, o)
		if out.String() != cleanStatus || r.Compacted || r.Passthrough != PassthroughSmallOutput || r.EmittedBytes != int64(len(cleanStatus)) {
			t.Errorf("%+v: out=%q compacted=%v passthrough=%q emitted=%d, want the raw output", o, out.String(), r.Compacted, r.Passthrough, r.EmittedBytes)
		}
	}
}

// A failed run returned as 0 (|| true) gets the exit note; whatever the size of
// the failure, the output is never longer than raw and EmittedBytes is exact.
func TestRunExitZeroNoteNeverLongerThanRaw(t *testing.T) {
	fail := "--- FAIL: TestCharge (0.01s)\n    billing_test.go:42: got 100, want 110\nFAIL\nFAIL\tgithub.com/x/internal/billing\t0.087s\nFAIL\n"
	for n := 1; n <= 12; n++ {
		raw := strings.Repeat("ok  \tgithub.com/x/internal/a\t0.1s\n", n) + fail
		fakeBin(t, "go", shellPrintf(raw)+"; exit 1")
		var out bytes.Buffer
		o := Options{Stdin: strings.NewReader(""), Stdout: &out, Stderr: &bytes.Buffer{}, Compact: true, ExitZero: true}
		r := Run([]string{"go", "test", "./..."}, o)
		if out.Len() > len(raw) {
			t.Errorf("n=%d: emitted %d bytes for %d raw: %q", n, out.Len(), len(raw), out.String())
		}
		if r.EmittedBytes != int64(out.Len()) {
			t.Errorf("n=%d: EmittedBytes=%d, wrote %d", n, r.EmittedBytes, out.Len())
		}
	}
}
