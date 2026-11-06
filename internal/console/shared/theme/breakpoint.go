// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package theme

// Breakpoint is a terminal width class that screens adapt to.
type Breakpoint int

// Width classes: Narrow is under 60 columns, Normal under 100, Wide from 100.
const (
	Narrow Breakpoint = iota
	Normal
	Wide
)

const (
	narrowBelow = 60
	wideFrom    = 100
)

// BreakpointFor classifies a terminal width in columns. An unknown width
// (zero or negative, before the first size message) counts as Normal.
func BreakpointFor(width int) Breakpoint {
	switch {
	case width <= 0:
		return Normal
	case width < narrowBelow:
		return Narrow
	case width < wideFrom:
		return Normal
	}
	return Wide
}
