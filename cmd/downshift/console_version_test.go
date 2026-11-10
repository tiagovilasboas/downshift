// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/console/surface"
)

// version prints its line in a terminal and never opens the console.
func TestVersionNeverOpensConsole(t *testing.T) {
	f := surface.Facts{Args: []string{"version"}, StdinTTY: true, StdoutTTY: true}
	if d := surface.Decide(f, consoleCommands); d.Console {
		t.Fatalf("version opened the console: %+v", d)
	}
}
