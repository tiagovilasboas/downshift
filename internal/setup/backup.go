// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package setup

import (
	"path/filepath"
	"sort"
	"strings"
)

// BackupInfix sits between a file name and the stamp of the backup setup
// keeps next to it.
const BackupInfix = ".downshift-bak-"

// LatestBackup returns the newest backup for path, and whether it is the
// "file did not exist" marker.
func LatestBackup(path string) (backup string, marker bool) {
	matches, _ := filepath.Glob(path + BackupInfix + "*")
	if len(matches) == 0 {
		return "", false
	}
	sort.Strings(matches)
	b := matches[len(matches)-1]
	return b, strings.HasSuffix(b, ".new")
}
