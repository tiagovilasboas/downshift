// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/paths"
)

func TestDesktopStepForEveryCallOfTheFamily(t *testing.T) {
	t.Setenv(paths.EnvStateDir, t.TempDir())
	read := []string{"agent-desktop", "snapshot", "--app", "Chat"}
	if desktopStep("", read) == nil {
		t.Error("a direct call without a session must get the step, scoped to its directory")
	}
	if desktopStep("abc", []string{"git", "status"}) != nil {
		t.Error("another family must get no delta")
	}
	t.Setenv(desktopDeltaEnv, "0")
	if desktopStep("abc", read) != nil {
		t.Error("the env switch must turn the delta off")
	}
	t.Setenv(desktopDeltaEnv, "")
	if desktopStep("abc", read) == nil {
		t.Error("a session read of agent-desktop must get the step")
	}
}
