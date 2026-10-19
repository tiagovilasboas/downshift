// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package paths

import (
	"os"
	"syscall"
)

// ownedByCaller reports whether info describes a file owned by the current user.
func ownedByCaller(info os.FileInfo) bool {
	st, ok := info.Sys().(*syscall.Stat_t)
	return ok && int(st.Uid) == os.Getuid()
}
