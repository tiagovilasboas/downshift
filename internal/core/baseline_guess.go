// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core

// HoldGuessedDownshift holds a downshift whose baseline was guessed (Claude
// Code settings, a Cursor parent model). The session may run another model
// (--model, /model), and only the small tier is surely cheaper than anything
// it can run, so a downshift on a guessed baseline is written only to that
// tier; any other target could be an upshift nobody sees. It sets
// Baseline.GuessHeld and reports true when the rewrite to target must not be
// written. Adapters call it; they do not hold this policy themselves.
func (d *Decision) HoldGuessedDownshift(target Model) bool {
	if !d.Baseline.Guessed || d.Verdict != VerdictDownshift || target.Tier == TierSmall {
		return false
	}
	d.Baseline.GuessHeld = true
	return true
}
