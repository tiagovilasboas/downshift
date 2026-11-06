// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// systemPathDirs are the PATH entries TestMain keeps: the OS system dirs and
// the dirs of the toolchain the tests themselves run (git, go, sh). Harness
// CLIs live elsewhere (homebrew, npm, ~/.local), so they drop out of PATH.
func systemPathDirs() []string {
	// The toolchain dirs come first: a CI runner can carry an older go in
	// /usr/bin, and the tests must build with the go running them.
	var dirs []string
	for _, tool := range []string{"go", "git", "sh"} {
		if p, err := exec.LookPath(tool); err == nil {
			dirs = append(dirs, filepath.Dir(p))
		}
	}
	if runtime.GOOS == "windows" {
		if root := os.Getenv("SystemRoot"); root != "" {
			dirs = append(dirs, filepath.Join(root, "System32"), root)
		}
	} else {
		dirs = append(dirs, "/usr/bin", "/bin", "/usr/sbin", "/sbin")
	}
	return dirs
}
