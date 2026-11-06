// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"

	"github.com/tiagovilasboas/downshift/internal/adapters/cursor"
	"github.com/tiagovilasboas/downshift/internal/shellwrap"
)

// cursorWire is Cursor's hook wire: one JSON line on stdout, exit 0 always.
// Routed spawns go out through rewriteWire unchanged. A shell command that
// Downshift does not rewrite is answered with the neutral `{}`, never an allow:
// an allow would switch off Cursor's own approval flow for a command Downshift
// does not own. The advise result is an agent_message with no permission.
type cursorWire struct {
	rewriteWire
	// nonShell is set by the parse seam only for a payload that parsed as a
	// non-shell preToolUse call. Anything else (a shell call, a payload that
	// did not parse) fails open neutral.
	nonShell *bool
}

func (w cursorWire) failOpen(cause, correlationID string) {
	if w.nonShell != nil && *w.nonShell {
		w.rewriteWire.failOpen(cause, correlationID)
		return
	}
	fmt.Println(`{}`)
}

func (w cursorWire) emit(out any, correlationID string) (int, error) {
	if advice, ok := out.(shellwrap.AdviseResult); ok {
		out = cursor.ShellAdvise(advice.Message)
	}
	return w.rewriteWire.emit(out, correlationID)
}
