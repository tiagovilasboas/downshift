// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package pick holds reusable Bubble Tea v2 choice components: List (one
// choice) and MultiSelect (many). They are models with Update and View; the
// caller owns the tea.Program and reads Done, Cancelled and the result.
package pick

// Item is one row. Selected is the preselection (MultiSelect only). A
// Disabled item cannot be chosen or toggled, keeps its Selected, and shows
// DisabledReason.
type Item struct {
	ID, Label, Hint string
	Selected        bool
	Disabled        bool
	DisabledReason  string
}
