// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/setup"
)

func shadowFixture(t *testing.T) (a, b, missing string) {
	t.Helper()
	root := t.TempDir()
	a, b = filepath.Join(root, "a"), filepath.Join(root, "b")
	for _, d := range []string{a, b} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return a, b, filepath.Join(root, "missing")
}

func writeBin(t *testing.T, dir, body string) string {
	t.Helper()
	p := filepath.Join(dir, setup.BinaryName())
	if err := os.WriteFile(p, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestCheckBinaryShadow(t *testing.T) {
	cases := []struct {
		name     string
		setup    func(t *testing.T, a, b string)
		wantWarn bool
	}{
		{"single copy", func(t *testing.T, a, b string) { writeBin(t, a, "one") }, false},
		{"no copy", func(t *testing.T, a, b string) {}, false},
		{"two distinct files", func(t *testing.T, a, b string) {
			writeBin(t, a, "old")
			writeBin(t, b, "new")
		}, true},
		{"symlink to same file", func(t *testing.T, a, b string) {
			real := writeBin(t, a, "one")
			if err := os.Symlink(real, filepath.Join(b, setup.BinaryName())); err != nil {
				t.Skipf("symlinks unavailable: %v", err)
			}
		}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a, b, missing := shadowFixture(t)
			tc.setup(t, a, b)
			env := doctorEnv{PathDirs: func() []string { return []string{missing, a, "", b} }}
			rep := &doctorReport{}
			checkBinaryShadow(rep, env)
			if !tc.wantWarn {
				if len(rep.Checks) != 0 {
					t.Fatalf("want no check, got %+v", rep.Checks)
				}
				return
			}
			if len(rep.Checks) != 1 {
				t.Fatalf("want 1 check, got %+v", rep.Checks)
			}
			c := rep.Checks[0]
			first, later := filepath.Join(a, setup.BinaryName()), filepath.Join(b, setup.BinaryName())
			if c.ID != "binary shadowed" || c.Status != doctorWarn {
				t.Fatalf("bad check %+v", c)
			}
			if !strings.HasPrefix(c.Detail, first+" runs first") || !strings.Contains(c.Detail, "shadowed: "+later) {
				t.Fatalf("order wrong in detail: %q", c.Detail)
			}
			if !strings.Contains(c.Fix, later+" "+first) {
				t.Fatalf("fix should link later over first: %q", c.Fix)
			}
		})
	}
}

func TestCheckBinaryShadowNilPathDirsSkips(t *testing.T) {
	rep := &doctorReport{}
	checkBinaryShadow(rep, doctorEnv{})
	if len(rep.Checks) != 0 {
		t.Fatalf("nil PathDirs must skip, got %+v", rep.Checks)
	}
}

func TestPathCopiesIgnoresNonExecutable(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("mode bits are not meaningful on Windows")
	}
	a, _, _ := shadowFixture(t)
	if err := os.WriteFile(filepath.Join(a, setup.BinaryName()), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if got := pathCopies([]string{a}); len(got) != 0 {
		t.Fatalf("non-executable counted: %v", got)
	}
}
