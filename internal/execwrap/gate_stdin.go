// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import (
	"io"
	"os"
)

// interactiveStdin reports whether r is a terminal a person may type into: a
// character device other than the null device. A pipe, a file, a reader in
// memory or the null device (how agent harnesses run commands) is not. When
// the null device cannot be told apart, a character device counts as a
// terminal, which keeps the old rule: a stderr byte releases the output.
func interactiveStdin(r io.Reader) bool {
	f, ok := r.(*os.File)
	if !ok || f == nil || f.Name() == os.DevNull {
		return false
	}
	info, err := f.Stat()
	if err != nil || info.Mode()&os.ModeCharDevice == 0 {
		return false
	}
	if null, err := os.Stat(os.DevNull); err == nil && os.SameFile(info, null) {
		return false
	}
	return true
}
