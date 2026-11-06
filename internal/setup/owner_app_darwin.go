// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

//go:build darwin

package setup

import (
	"context"
	"os/exec"
	"time"
)

// platformOwnerAppState lists the process executables with ps and looks for
// the app's binary by base name. Any failure is unknown, never "stopped".
func platformOwnerAppState(app string) OwnerAppState {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "ps", "-axo", "comm=").Output()
	if err != nil {
		return OwnerAppUnknown
	}
	if processListHas(string(out), app) {
		return OwnerAppRunning
	}
	return OwnerAppStopped
}
