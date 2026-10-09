// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

//go:build !unix

package sensor

import (
	"fmt"
	"os"
)

func lockFile(f *os.File) error {
	return fmt.Errorf("sensor file lock is unsupported on this platform")
}

func unlockFile(f *os.File) error {
	return nil
}
