// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package execwrap

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/compressor"
)

// fakeBin writes an executable shell script named name into a fresh PATH
// directory, so Run sees argv[0] == "go" or "git" without the real tools.
func fakeBin(t *testing.T, name, script string) {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte("#!/bin/sh\n"+script+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func run(t *testing.T, argv []string, compact bool) (Result, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	r := Run(argv, Options{Stdin: strings.NewReader(""), Stdout: &out, Stderr: &errb, Compact: compact})
	return r, out.String(), errb.String()
}

// passingSuite is a realistic all-ok run: enough packages that the compressor
// summary is strictly shorter than the input.
var passingSuite = func() string {
	var b strings.Builder
	for _, p := range []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j"} {
		b.WriteString("ok  \tgithub.com/x/internal/" + p + "\t0.1s\n")
	}
	return b.String()
}()

// compactedWant is what a compacted run prints: the compressor's own summary
// plus the marker line.
func compactedWant(raw string) string {
	out := compressor.CompressExit([]byte(raw), compressor.ModeSafe, 0).Output
	return string(out) + Marker(len(raw), len(out))
}

func itoa(n int) string {
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	if len(b) == 0 {
		return "0"
	}
	return string(b)
}
