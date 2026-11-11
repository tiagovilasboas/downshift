// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package selfupdate

import "os"

// swap renames tmp over target; a running binary may be replaced on POSIX.
func swap(target, tmp string) error {
	return os.Rename(tmp, target)
}
