// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package paths

import "os"

// Windows has no POSIX modes to tighten; ownership is not checked.
func ownedByCaller(os.FileInfo) bool { return false }
