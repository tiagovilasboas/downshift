// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package testenv is the single cross-OS layer for test environment setup.
// It exists so no test hard-codes a per-OS home variable: os.UserHomeDir reads
// HOME on Unix and USERPROFILE on Windows, so a test that only sets HOME passes
// on Linux/macOS and silently uses the real home on Windows. SetHome sets both.
package testenv

import "testing"

// SetHome points os.UserHomeDir at dir for the duration of the test on every
// OS: HOME for Unix, USERPROFILE (and HOMEDRIVE/HOMEPATH) for Windows. Use it
// instead of t.Setenv("HOME", dir).
func SetHome(t *testing.T, dir string) {
	t.Helper()
	t.Setenv("HOME", dir)
	// Windows: os.UserHomeDir prefers USERPROFILE; some paths fall back to
	// HOMEDRIVE+HOMEPATH. Set all three so the test home wins regardless.
	t.Setenv("USERPROFILE", dir)
	t.Setenv("HOMEDRIVE", "")
	t.Setenv("HOMEPATH", dir)
}
