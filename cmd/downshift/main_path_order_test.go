// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os/exec"
	"path/filepath"
	"testing"
)

// The go that runs the tests must come first on the test PATH: a CI runner's
// /usr/bin can hold an older go that cannot build this module.
func TestTestPathPutsGoFirst(t *testing.T) {
	p, err := exec.LookPath("go")
	if err != nil {
		t.Skipf("go not on PATH: %v", err)
	}
	dirs := systemPathDirs()
	if len(dirs) == 0 || dirs[0] != filepath.Dir(p) {
		t.Fatalf("first test PATH dir = %v, want %s", dirs, filepath.Dir(p))
	}
}
