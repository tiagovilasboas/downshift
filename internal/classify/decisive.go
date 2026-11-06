// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package classify

// Decisive reports a classification routing may act on: the top class
// clearly won, a content signal fired, and no risk rule lifted or cleared it.
func (c Classification) Decisive() bool {
	return c.Confident && !c.NoSignal && !c.RiskFloor && !c.RiskAbstain
}
