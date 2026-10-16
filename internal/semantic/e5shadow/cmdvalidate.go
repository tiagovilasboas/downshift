// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package e5shadow

import (
	"os"
	"path/filepath"
	"strings"
)

// CmdTrusted reports whether cmdline looks like a local embed helper (not a
// network shell one-liner). Used by shadow observation, fuse, and doctor.
// Symlinks are rejected (Lstat); glued -c/-e flags are rejected.
func CmdTrusted(cmdline string) bool {
	parts := strings.Fields(strings.TrimSpace(cmdline))
	if len(parts) == 0 {
		return false
	}
	lower := strings.ToLower(cmdline)
	if strings.Contains(lower, " -c ") || strings.HasSuffix(lower, " -c") {
		return false
	}
	banned := map[string]struct{}{
		"curl": {}, "wget": {}, "nc": {}, "bash": {}, "sh": {}, "zsh": {},
	}
	for _, p := range parts {
		base := strings.ToLower(filepath.Base(p))
		if _, ok := banned[base]; ok {
			return false
		}
		if isEvalFlag(p) {
			return false
		}
	}
	exe := parts[0]
	if filepath.IsAbs(exe) {
		return regularFile(exe)
	}
	if (exe == "python3" || exe == "python") && len(parts) >= 2 && filepath.IsAbs(parts[1]) {
		return regularFile(parts[1])
	}
	return false
}

func isEvalFlag(p string) bool {
	if strings.HasPrefix(p, "--") {
		return false
	}
	return p == "-c" || p == "-e" || strings.HasPrefix(p, "-c") || strings.HasPrefix(p, "-e")
}

func regularFile(path string) bool {
	info, err := os.Lstat(path)
	return err == nil && info.Mode()&os.ModeSymlink == 0 && info.Mode().IsRegular()
}
