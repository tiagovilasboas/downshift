// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package opencode

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

// managedDir is the seam for the managed config dir; tests replace it so
// they never read the real system directory.
var managedDir = systemManagedDir

func managedConfigDir() string { return managedDir() }

// managedPrefs is the seam for the macOS managed-preferences folder; tests
// replace it. It is empty where the platform has none.
var managedPrefs = systemManagedPrefs

// managedPreferenceFile is the MDM profile OpenCode reads on macOS.
const managedPreferenceFile = "ai.opencode.managed.plist"

// managedPreferencePresent reports whether an MDM profile for OpenCode exists
// at the managed-preferences root or under any user folder in it. Such a
// profile can carry permission rules this check does not parse, so its
// presence alone keeps the command unchanged.
func managedPreferencePresent() bool {
	root := managedPrefs()
	if root == "" {
		return false
	}
	candidates := []string{filepath.Join(root, managedPreferenceFile)}
	perUser, err := filepath.Glob(filepath.Join(root, "*", managedPreferenceFile))
	if err != nil {
		return true
	}
	for _, path := range append(candidates, perUser...) {
		if _, err := os.Lstat(path); !errors.Is(err, fs.ErrNotExist) {
			return true
		}
	}
	return false
}
