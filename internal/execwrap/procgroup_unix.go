// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package execwrap

import (
	"os"
	"os/exec"
	"syscall"
)

// ownProcessGroup starts the child in its own process group, so a stop or a
// timeout reaches the program and everything it spawned (go test children),
// and nothing is left orphaned when the wrapper is told to stop.
func ownProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// signalChild delivers sig to the child's whole process group, falling back
// to the child alone.
func signalChild(cmd *exec.Cmd, sig os.Signal) {
	if s, ok := sig.(syscall.Signal); ok && cmd.Process != nil {
		if syscall.Kill(-cmd.Process.Pid, s) == nil {
			return
		}
	}
	_ = cmd.Process.Signal(sig)
}
