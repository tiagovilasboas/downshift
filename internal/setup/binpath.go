// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package setup

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// UnsafeBinaryReason reports why exe is a bad hook target, or "" when it is
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
func UnsafeBinaryReason(exe, tmp string) string {
	exe = ResolvePath(exe)
	if tmp != "" && within(exe, ResolvePath(tmp)) {
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

func ResolvePath(p string) string {
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

// GuardHookBinary refuses to write hooks that point at a binary which will
// vanish or which another user can replace. A dry run and --undo only warn
// (they write no hook command); --allow-unsafe-binary overrides.
func GuardHookBinary(o Options, allow bool, tmp string) int {
	why := UnsafeBinaryReason(o.Binary, tmp)
	if why == "" {
		return 0
	}
	msg := "setup: this binary (" + o.Binary + ") is not a safe hook target: " + why
	if allow || o.DryRun || o.Undo {
		fmt.Fprintln(o.Err, "warning: "+msg)
		return 0
	}
	fmt.Fprintln(o.Err, msg+".\nInstall downshift to a stable directory (e.g. `go install`, Homebrew, /usr/local/bin) and run setup from there, or pass --allow-unsafe-binary.")
	return 1
}
