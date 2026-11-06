// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"io"
	"os"
	"os/exec"
)

// runTests runs go test -count=1 ./... (no test cache) with its output
// streamed, since a full run takes minutes.
func runTests(stdout, stderr io.Writer) error {
	cmd := exec.Command("go", "test", "-count=1", "./...")
	cmd.Env = os.Environ()
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}
