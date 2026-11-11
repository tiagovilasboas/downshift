// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package selfupdate

import (
	"fmt"
	"os"
)

// swap moves the running target aside (Windows refuses to overwrite a running
// .exe but allows renaming it), then moves tmp into place. The .old copy is
// removed on the next update, once nothing runs it.
func swap(target, tmp string) error {
	old := OldCopy(target)
	_ = os.Remove(old)
	if err := os.Rename(target, old); err != nil {
		return fmt.Errorf("moving the running binary aside: %w", err)
	}
	if err := os.Rename(tmp, target); err != nil {
		_ = os.Rename(old, target)
		return fmt.Errorf("moving the new binary into place: %w", err)
	}
	return nil
}
