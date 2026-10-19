// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package execwrap

import (
	"os"
	"os/exec"
)

func ownProcessGroup(*exec.Cmd) {}

func signalChild(cmd *exec.Cmd, sig os.Signal) { _ = cmd.Process.Signal(sig) }
