// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

//go:build !darwin && !windows

package opencode

// systemManagedDir is where OpenCode reads the managed (system) config.
func systemManagedDir() string { return "/etc/opencode" }

// systemManagedPrefs: only macOS has managed preferences.
func systemManagedPrefs() string { return "" }
