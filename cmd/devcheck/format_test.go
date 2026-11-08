// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestToolchainGofmtPath(t *testing.T) {
	for _, c := range []struct{ goos, want string }{
		{"linux", filepath.Join("root", "bin", "gofmt")},
		{"darwin", filepath.Join("root", "bin", "gofmt")},
		{"windows", filepath.Join("root", "bin", "gofmt.exe")},
	} {
		if got := toolchainGofmt("root", c.goos); got != c.want {
			t.Errorf("toolchainGofmt(root, %s) = %q, want %q", c.goos, got, c.want)
		}
	}
}

// The format stage must run the gofmt of the toolchain the go command uses,
// the one CI gets from go.mod, not whatever gofmt is first on PATH.
func TestGofmtBinIsTheToolchainGofmt(t *testing.T) {
	out, err := exec.Command("go", "env", "GOROOT").Output()
	if err != nil {
		t.Skipf("go env GOROOT: %v", err)
	}
	root := strings.TrimSpace(string(out))
	got := gofmtBin()
	if filepath.Dir(filepath.Dir(got)) != filepath.Clean(root) {
		t.Fatalf("gofmtBin() = %q, want a gofmt under GOROOT %q", got, root)
	}
	if _, err := os.Stat(got); err != nil {
		t.Fatalf("gofmtBin() = %q does not exist: %v", got, err)
	}
}
