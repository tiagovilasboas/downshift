// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package opencode

import (
	"os"
	"path/filepath"
)

// systemManagedDir is where OpenCode reads the managed (system) config.
func systemManagedDir() string {
	base := os.Getenv("ProgramData")
	if base == "" {
		base = `C:\ProgramData`
	}
	return filepath.Join(base, "opencode")
}

// systemManagedPrefs: only macOS has managed preferences.
func systemManagedPrefs() string { return "" }
