// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"github.com/tiagovilasboas/downshift/internal/adapters/cursor"
	"github.com/tiagovilasboas/downshift/internal/shellwrap"
)

// cursorWire is Cursor's hook wire: one JSON line on stdout, exit 0 always.
// Routed spawns and preToolUse rewrites go out through rewriteWire unchanged.
// The shell advise result becomes a beforeShellExecution allow carrying
// agent_message, so Cursor never blocks a command for it.
type cursorWire struct{ rewriteWire }

func (w cursorWire) emit(out any, correlationID string) (int, error) {
	if advice, ok := out.(shellwrap.AdviseResult); ok {
		out = cursor.ShellAdvise(advice.Message)
	}
	return w.rewriteWire.emit(out, correlationID)
}
