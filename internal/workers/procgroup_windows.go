// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package workers

import "os/exec"

func ownProcessGroup(*exec.Cmd) {}

// killGroup stops the worker alone: Windows has no process group to signal here.
func killGroup(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	return cmd.Process.Kill()
}
