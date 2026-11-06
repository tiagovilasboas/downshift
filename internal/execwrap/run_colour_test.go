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

// fakeOutput makes `go` print exactly out.
func fakeOutput(t *testing.T, out string) {
	t.Helper()
	src := filepath.Join(t.TempDir(), "out.txt")
	if err := os.WriteFile(src, []byte(out), 0o600); err != nil {
		t.Fatal(err)
	}
	fakeBin(t, "go", "cat "+src)
}

// Test runners colour their output even when it is piped. A coloured clean run
// compacts like a plain one, and the summary carries no escape codes.
func TestRun_CompactsColouredCleanRun(t *testing.T) {
	fakeOutput(t, strings.ReplaceAll(passingSuite, "ok  \t", "\x1b[32mok\x1b[0m  \t"))
	r, out, _ := run(t, []string{"go", "test", "./..."}, true)
	if !r.Compacted || strings.Contains(out, "\x1b") || !strings.Contains(out, "10 packages ok") {
		t.Fatalf("compacted=%v out=%q", r.Compacted, out)
	}
}

// Any other terminal escape keeps the bytes raw.
func TestRun_KeepsTerminalRenderingRaw(t *testing.T) {
	in := "\x1b[2K" + passingSuite
	fakeOutput(t, in)
	r, out, _ := run(t, []string{"go", "test", "./..."}, true)
	if r.Compacted || out != in {
		t.Fatalf("compacted=%v out=%q", r.Compacted, out)
	}
}
