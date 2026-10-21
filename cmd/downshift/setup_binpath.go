// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// unsafeBinaryReason reports why exe is a bad hook target, or "" when it is
// fine. setup stores exe in the harness hook config and every prompt then runs
// it, so a binary that will disappear or that another user can replace must
// not be written there:
//   - under tmp (go run, go test, a download unpacked to /tmp): cleaned or
//     replaced later;
//   - under a Go build directory (a path element starting with go-build);
//   - in a world-writable directory without the sticky bit: anyone can swap
//     the file. Group-writable is accepted because Homebrew's bin directory is
//     group-writable by admin on macOS.
//
// tmp is os.TempDir() in production; tests pass their own root.
func unsafeBinaryReason(exe, tmp string) string {
	exe = resolvePath(exe)
	if tmp != "" && within(exe, resolvePath(tmp)) {
		return "it is under the temporary directory " + tmp + ", which is cleaned"
	}
	for _, part := range strings.Split(filepath.ToSlash(filepath.Dir(exe)), "/") {
		if strings.HasPrefix(part, "go-build") {
			return "it is in a Go build directory (go run or go test), which is deleted"
		}
	}
	if runtime.GOOS == "windows" {
		return ""
	}
	dir := filepath.Dir(exe)
	if fi, err := os.Stat(dir); err == nil && fi.Mode().Perm()&0o002 != 0 && fi.Mode()&os.ModeSticky == 0 {
		return "its directory " + dir + " is writable by every user, so anyone can replace it"
	}
	return ""
}

func resolvePath(p string) string {
	if abs, err := filepath.Abs(p); err == nil {
		p = abs
	}
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return r
	}
	return filepath.Clean(p)
}

// within reports whether p is root or below it.
func within(p, root string) bool {
	rel, err := filepath.Rel(root, p)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
