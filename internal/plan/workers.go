// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package plan

// Worker modes, as the server resolved them at start.
const (
	WorkersOff   = "off"
	WorkersRead  = "read"
	WorkersWrite = "write"
)

// Workers is what the plan needs to know about the MCP worker tools: the mode
// the server runs in and the worker agents it can start. The zero value is
// off, so a caller that passes nothing gets no delegate steps.
type Workers struct {
	Mode   string
	Agents []string
}

// Enabled reports whether a delegate step for agent can run.
func (w Workers) Enabled(agent string) bool {
	if w.Mode != WorkersRead && w.Mode != WorkersWrite {
		return false
	}
	for _, a := range w.Agents {
		if a == agent {
			return true
		}
	}
	return false
}

// WriteEnabled reports whether a delegate step may ask for mode write.
func (w Workers) WriteEnabled() bool { return w.Mode == WorkersWrite }
