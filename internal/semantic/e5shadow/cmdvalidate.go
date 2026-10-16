// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package e5shadow

import (
	"os"
	"path/filepath"
	"strings"
)

// CmdTrusted reports whether cmdline looks like a local embed helper (not a
// network shell one-liner). Used by shadow observation and doctor.
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
	}
	exe := parts[0]
	if filepath.IsAbs(exe) {
		info, err := os.Stat(exe)
		return err == nil && info.Mode().IsRegular()
	}
	if (exe == "python3" || exe == "python") && len(parts) >= 2 && filepath.IsAbs(parts[1]) {
		info, err := os.Stat(parts[1])
		return err == nil && info.Mode().IsRegular()
	}
	return false
}
