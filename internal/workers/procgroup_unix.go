// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package workers

import (
	"errors"
	"os/exec"
	"syscall"
)

// ownProcessGroup starts the worker in its own process group, so stopping it
// reaches the programs it started (test runs, builds) and nothing is left
// running after a cancel or a budget expiry.
func ownProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// adoptProcess is a no-op here: the process group is set before Start.
func adoptProcess(*exec.Cmd) {}

// releaseProcess is a no-op here: a process group holds no handle.
func releaseProcess(*exec.Cmd) {}

// killGroup stops the worker's whole process group, falling back to the worker
// alone. A group that is already gone is not an error.
func killGroup(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	if err == nil || errors.Is(err, syscall.ESRCH) {
		return nil
	}
	return cmd.Process.Kill()
}
