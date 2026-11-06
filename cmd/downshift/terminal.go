// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"io"
	"os"

	"github.com/mattn/go-isatty"
)

// isInteractiveTerminal reports whether in is a terminal a person types into.
func isInteractiveTerminal(in io.Reader) bool {
	f, ok := in.(*os.File)
	if !ok || f == nil {
		return false
	}
	return isatty.IsTerminal(f.Fd()) || isatty.IsCygwinTerminal(f.Fd())
}
