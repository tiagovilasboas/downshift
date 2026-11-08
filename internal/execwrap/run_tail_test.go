// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package execwrap

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/compressor"
)

func TestParseTailFlags(t *testing.T) {
	var o Options
	rest := ParseTailFlags([]string{FlagExitZero, FlagDiscardStderr, "--", "go", "test"}, &o)
	if !o.DiscardStderr || !o.ExitZero || !reflect.DeepEqual(rest, []string{"--", "go", "test"}) {
		t.Fatalf("o=%+v rest=%q", o, rest)
	}
	var none Options
	if rest := ParseTailFlags([]string{"--", FlagExitZero}, &none); none.ExitZero || len(rest) != 2 {
		t.Fatal("a flag after -- is argv, not a tail flag")
	}
}

// 2>/dev/null: the program's stderr reaches nobody; an output too small to
// compact is the program's stdout byte for byte, with no note.
func TestRunDiscardStderr(t *testing.T) {
	fakeBin(t, "go", "echo out; echo secret-err >&2")
	var out, errb bytes.Buffer
	r := Run([]string{"go", "test"}, Options{Stdin: strings.NewReader(""), Stdout: &out, Stderr: &errb, Compact: true, DiscardStderr: true})
	if r.ExitCode != 0 || errb.Len() != 0 || out.String() != "out\n" {
		t.Fatalf("exit=%d out=%q err=%q, want exactly the program's stdout", r.ExitCode, out.String(), errb.String())
	}
}

// || true: the wrapper exits 0 and the result keeps the real code; an
// uncompacted output carries no note.
func TestRunExitZeroKeepsRealCode(t *testing.T) {
	fakeBin(t, "go", "echo boom; exit 3")
	var out bytes.Buffer
	o := Options{Stdin: strings.NewReader(""), Stdout: &out, Stderr: &bytes.Buffer{}, Compact: true, ExitZero: true}
	r := Run([]string{"go", "test"}, o)
	if r.ExitCode != 3 || ShellExit(o, r.ExitCode) != 0 || ShellExit(Options{}, r.ExitCode) != 3 {
		t.Fatalf("exit=%d shell=%d", r.ExitCode, ShellExit(o, r.ExitCode))
	}
	if out.String() != "boom\n" {
		t.Fatalf("out = %q, want exactly the program's stdout", out.String())
	}
}

// Notes follow only a compacted output. Raw output under each flag, with
// compaction on or off (kill switch, nested call), is the program's stdout
// byte for byte; `git rev-parse HEAD 2>/dev/null` prints just the hash.
func TestRunTailNotesOnlyWhenCompacted(t *testing.T) {
	const hash = "0123456789abcdef0123456789abcdef01234567\n"
	fakeBin(t, "git", "printf '"+strings.TrimSuffix(hash, "\n")+"\\n'; echo err >&2; exit 0")
	for _, compact := range []bool{true, false} {
		for name, o := range map[string]Options{"discard": {DiscardStderr: true}, "exit-zero": {ExitZero: true}, "both": {DiscardStderr: true, ExitZero: true}} {
			var out bytes.Buffer
			o.Stdin, o.Stdout, o.Stderr, o.Compact = strings.NewReader(""), &out, &bytes.Buffer{}, compact
			if r := Run([]string{"git", "rev-parse", "HEAD"}, o); r.Compacted || out.String() != hash {
				t.Errorf("%s compact=%v: out=%q compacted=%v, want just the hash", name, compact, out.String(), r.Compacted)
			}
		}
	}
	fakeBin(t, "go", shellPrintf(failedSuite)+"; echo hidden >&2; exit 1")
	var out bytes.Buffer
	o := Options{Stdin: strings.NewReader(""), Stdout: &out, Stderr: &bytes.Buffer{}, Compact: true, DiscardStderr: true, ExitZero: true}
	r := Run([]string{"go", "test", "./..."}, o)
	if !r.Compacted {
		t.Fatalf("failed suite not compacted (passthrough %q): %q", r.Passthrough, out.String())
	}
	body := compressor.CompressFailed([]byte(failedSuite), compressor.FormatGoTest).Output
	want := string(body) + Marker(len(failedSuite), len(body)) +
		"[downshift compact: stderr discarded (2>/dev/null)]\n[downshift compact: exit 1, returned 0 (|| true)]\n"
	if out.String() != want {
		t.Fatalf("compacted output\n got: %q\nwant: %q", out.String(), want)
	}
}
