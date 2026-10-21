// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func binIn(t *testing.T, dir string, mode os.FileMode) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, mode); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
	p := filepath.Join(dir, "downshift")
	if err := os.WriteFile(p, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestUnsafeBinaryReason(t *testing.T) {
	root := t.TempDir()
	tmp := filepath.Join(root, "tmp")
	if err := os.MkdirAll(tmp, 0o755); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		exe  string
		want string // substring of the reason; "" means safe
	}{
		{"stable dir", binIn(t, filepath.Join(root, "usr", "bin"), 0o755), ""},
		{"temp dir", binIn(t, filepath.Join(tmp, "dl"), 0o755), "temporary directory"},
		{"go run", binIn(t, filepath.Join(root, "cache", "go-build123", "b001", "exe"), 0o755), "Go build"},
	}
	if runtime.GOOS != "windows" {
		cases = append(cases,
			struct{ name, exe, want string }{"world-writable", binIn(t, filepath.Join(root, "shared"), 0o777), "every user"},
			struct{ name, exe, want string }{"sticky world-writable", binIn(t, filepath.Join(root, "sticky"), 0o777|os.ModeSticky), ""},
			struct{ name, exe, want string }{"group-writable (Homebrew)", binIn(t, filepath.Join(root, "brew"), 0o775), ""},
		)
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := unsafeBinaryReason(tc.exe, tmp)
			if (tc.want == "") != (got == "") || !strings.Contains(got, tc.want) {
				t.Fatalf("unsafeBinaryReason(%s) = %q, want %q", tc.exe, got, tc.want)
			}
		})
	}
	// A symlink in a stable directory that points into tmp is still unsafe.
	link := filepath.Join(root, "usr", "bin", "linked")
	if err := os.Symlink(binIn(t, filepath.Join(tmp, "x"), 0o755), link); err == nil {
		if unsafeBinaryReason(link, tmp) == "" {
			t.Error("symlink into tmp must be unsafe")
		}
	}
}

func TestGuardHookBinary(t *testing.T) {
	tmp := t.TempDir()
	exe := binIn(t, filepath.Join(tmp, "build"), 0o755)
	for _, tc := range []struct {
		name  string
		o     setupOptions
		allow bool
		code  int
	}{
		{"refuses", setupOptions{}, false, 1},
		{"override", setupOptions{}, true, 0},
		{"dry run warns", setupOptions{DryRun: true}, false, 0},
		{"undo warns", setupOptions{Undo: true}, false, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var errw bytes.Buffer
			tc.o.Binary, tc.o.Err = exe, &errw
			if code := guardHookBinary(tc.o, tc.allow, tmp); code != tc.code {
				t.Fatalf("code %d, want %d (%s)", code, tc.code, errw.String())
			}
			if !strings.Contains(errw.String(), "not a safe hook target") {
				t.Fatalf("no explanation: %q", errw.String())
			}
		})
	}
	var errw bytes.Buffer
	safe := binIn(t, filepath.Join(t.TempDir(), "bin"), 0o755)
	if code := guardHookBinary(setupOptions{Binary: safe, Err: &errw}, false, filepath.Join(tmp, "other")); code != 0 || errw.Len() != 0 {
		t.Fatalf("safe binary: code %d, %q", code, errw.String())
	}
}
