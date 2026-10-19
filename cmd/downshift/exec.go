// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/tiagovilasboas/downshift/internal/execwrap"
)

// runExec is `downshift exec -- <program> [args...]`: run the program with no
// shell, keep its exit code, and compact only clean output of an allowlisted
// family (internal/execwrap). DOWNSHIFT_EXEC=0 runs it as a plain
// passthrough. The exec log records the family, byte counts and exit class
// only; a log failure changes nothing the command prints or returns.
func runExec(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "--" {
		fmt.Fprintln(stderr, "usage: downshift exec -- <program> [args...]")
		return 2
	}
	res := execwrap.Run(args[1:], execwrap.Options{
		Stdin:   stdin,
		Stdout:  stdout,
		Stderr:  stderr,
		Compact: os.Getenv(execEnvKillSwitch) != "0" && os.Getenv(execwrap.DepthEnv) == "",
	})
	if len(args) > 1 {
		if path, err := execwrap.EventsPath(); err == nil {
			_ = execwrap.Append(path, execwrap.EventFrom(res, time.Now()))
		}
	}
	return res.ExitCode
}
