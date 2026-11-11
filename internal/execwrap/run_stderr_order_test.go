// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package execwrap

import (
	"bytes"
	"strings"
	"testing"
)

// writeSplitSuite fakes a go binary that writes the first part of the passing
// suite on stdout, a note on stderr, then the rest of the suite on stdout.
func writeSplitSuite(t *testing.T, note string) {
	t.Helper()
	esc := func(s string) string { return strings.ReplaceAll(s, "\t", "\\t") }
	half := len(passingSuite) / 2
	fakeBin(t, "go", "printf '"+esc(passingSuite[:half])+"'; echo '"+note+"' >&2; printf '"+esc(passingSuite[half:])+"'")
}

// go test is a stderrIsReport family: its stderr is merged into the held
// report in write order, so a note written between two stdout chunks is kept
// verbatim inside the compacted output, exactly once, and the program's own
// stderr stream stays empty. Nothing is lost and the exit code is the program's.
func TestRun_CompactedMergedStderrKeptOnce(t *testing.T) {
	const note = "note: held-stderr-marker"
	writeSplitSuite(t, note)

	var out, errb bytes.Buffer
	r := Run([]string{"go", "test", "./..."}, Options{Stdin: strings.NewReader(""), Stdout: &out, Stderr: &errb, Compact: true})
	if r.ExitCode != 0 || !r.Compacted {
		t.Fatalf("result %+v", r)
	}
	if errb.Len() != 0 || strings.Count(out.String(), note+"\n") != 1 {
		t.Fatalf("stdout=%q stderr=%q", out.String(), errb.String())
	}
	if !strings.Contains(out.String(), "PASS (10 packages ok)") {
		t.Fatalf("summary missing: %q", out.String())
	}
	if r.RawBytes != int64(len(passingSuite)+len(note)+1) || r.EmittedBytes != int64(out.Len()) {
		t.Fatalf("raw=%d emitted=%d, printed %d", r.RawBytes, r.EmittedBytes, out.Len())
	}
}

// git status holds its stderr apart: the compacted stdout is written first,
// then the held stderr whole on the stderr writer, exactly once. The exit
// code is the program's and no byte is lost. With one shared writer the held
// stderr is the last thing written.
func TestRun_CompactedHeldStderrFollowsStdout(t *testing.T) {
	const note = "warning: held-stderr-marker"
	fakeBin(t, "git", "printf 'On branch main\\nYour branch is up to date with origin/main.\\n'; echo '"+note+"' >&2; printf '\\nnothing to commit, working tree clean\\n'")

	var out, errb bytes.Buffer
	r := Run([]string{"git", "status"}, Options{Stdin: strings.NewReader(""), Stdout: &out, Stderr: &errb, Compact: true})
	if r.ExitCode != 0 || !r.Compacted {
		t.Fatalf("result %+v", r)
	}
	if errb.String() != note+"\n" || strings.Contains(out.String(), "held-stderr-marker") {
		t.Fatalf("stdout=%q stderr=%q", out.String(), errb.String())
	}
	if !strings.HasPrefix(out.String(), "On branch main (working tree clean)\n") {
		t.Fatalf("stdout = %q", out.String())
	}
	if r.EmittedBytes != int64(out.Len()+errb.Len()) {
		t.Fatalf("emitted=%d, printed %d", r.EmittedBytes, out.Len()+errb.Len())
	}

	var merged lockedBuf
	Run([]string{"git", "status"}, Options{Stdin: strings.NewReader(""), Stdout: &merged, Stderr: &merged, Compact: true})
	if got := merged.b.String(); !strings.HasSuffix(got, note+"\n") || strings.Count(got, note) != 1 {
		t.Fatalf("merged = %q", got)
	}
}
