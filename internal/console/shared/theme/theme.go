// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package theme holds the console's colour roles. Colours are the 16 ANSI
// slots, so the user's terminal theme picks the actual tone on dark and light
// backgrounds. Every state also carries a symbol, so nothing is lost when
// colour is off (NO_COLOR, or a terminal without colour).
package theme

import (
	"os"

	"charm.land/lipgloss/v2"
)

// Symbols that carry state without colour.
const (
	SymOK      = "✓"
	SymWarn    = "!"
	SymFail    = "✗"
	SymMuted   = "·"
	SymPointer = "›"
)

// Theme is the set of styles every screen draws with.
type Theme struct {
	Accent, Info, OK, Warn, Fail, Muted, Bold, Selected lipgloss.Style
	NoColor                                             bool
}

// New returns the theme for the current environment.
func New() Theme { return build(os.Getenv("NO_COLOR") != "") }

func build(noColor bool) Theme {
	base := lipgloss.NewStyle()
	t := Theme{Bold: base.Bold(true), NoColor: noColor}
	if noColor {
		t.Accent, t.Info, t.OK, t.Warn, t.Fail, t.Muted = base, base, base, base, base, base
		t.Selected = base.Reverse(true)
		return t
	}
	t.Accent = base.Foreground(lipgloss.Color("4")).Bold(true)
	t.Info = base.Foreground(lipgloss.Color("6"))
	t.OK = base.Foreground(lipgloss.Color("2"))
	t.Warn = base.Foreground(lipgloss.Color("3"))
	t.Fail = base.Foreground(lipgloss.Color("1"))
	t.Muted = base.Foreground(lipgloss.Color("8"))
	t.Selected = base.Reverse(true)
	return t
}

// Status renders a check status ("ok", "warn", "fail") as symbol plus colour.
func (t Theme) Status(status string) string {
	switch status {
	case "ok":
		return t.OK.Render(SymOK)
	case "warn":
		return t.Warn.Render(SymWarn)
	case "fail":
		return t.Fail.Render(SymFail)
	}
	return t.Muted.Render(SymMuted)
}
