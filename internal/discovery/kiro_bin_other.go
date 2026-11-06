// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

//go:build !darwin

package discovery

// bundledKiroCLI has no known location outside macOS yet. The override and
// PATH still apply; the Windows and Linux bundle paths are not verified.
func bundledKiroCLI() string { return "" }
