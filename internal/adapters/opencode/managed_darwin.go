// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

//go:build darwin

package opencode

// systemManagedDir is where OpenCode reads the managed (system) config.
func systemManagedDir() string { return "/Library/Application Support/opencode" }

// systemManagedPrefs is the MDM managed-preferences folder.
func systemManagedPrefs() string { return "/Library/Managed Preferences" }
