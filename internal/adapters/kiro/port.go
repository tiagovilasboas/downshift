// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package kiro

import "github.com/tiagovilasboas/downshift-ai/internal/hookport"

// Port returns the observation port for Kiro IDE post-spawn events.
func Port() hookport.Port {
	return hookport.Port{ID: harnessID, ParseChildEnd: ParseChildEnd}
}
