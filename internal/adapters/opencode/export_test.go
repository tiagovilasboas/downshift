// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package opencode

// SetManagedDirForTest points the managed config dir at dir for one test.
func SetManagedDirForTest(dir string) (restore func()) {
	old := managedDir
	managedDir = func() string { return dir }
	return func() { managedDir = old }
}

// SetManagedPrefsForTest points the macOS managed-preferences folder at dir
// for one test.
func SetManagedPrefsForTest(dir string) (restore func()) {
	old := managedPrefs
	managedPrefs = func() string { return dir }
	return func() { managedPrefs = old }
}
