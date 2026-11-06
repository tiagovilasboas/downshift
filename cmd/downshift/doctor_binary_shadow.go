// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/tiagovilasboas/downshift/internal/setup"
)

// pathCopies lists the distinct downshift executables found in dirs, in PATH
// order. Symbolic links to the same file count once; missing dirs are skipped.
// Nothing is executed.
func pathCopies(dirs []string) []string {
	var out []string
	seen := map[string]bool{}
	for _, dir := range dirs {
		if dir == "" {
			continue
		}
		p := filepath.Join(dir, setup.BinaryName())
		fi, err := os.Stat(p)
		if err != nil || fi.IsDir() {
			continue
		}
		if runtime.GOOS != "windows" && fi.Mode()&0o111 == 0 {
			continue
		}
		real := resolvedPath(p)
		if seen[real] {
			continue
		}
		seen[real] = true
		out = append(out, p)
	}
	return out
}

// checkBinaryShadow warns when more than one distinct downshift is on PATH:
// the first one wins, so an install into a later directory never takes effect.
func checkBinaryShadow(rep *doctorReport, env doctorEnv) {
	if env.PathDirs == nil {
		return
	}
	copies := pathCopies(env.PathDirs())
	if len(copies) < 2 {
		return
	}
	first, shadowed := copies[0], copies[1:]
	rep.add(doctorCheck{ID: "binary shadowed", Status: doctorWarn,
		Detail: fmt.Sprintf("%s runs first on PATH; shadowed: %s. An install into a shadowed directory does not take effect",
			first, strings.Join(shadowed, ", ")),
		Fix: fmt.Sprintf("compare `%s version` with `%s version`; keep one. If the shadowed one is newer: ln -sf %s %s",
			first, shadowed[0], shadowed[0], first)})
}
