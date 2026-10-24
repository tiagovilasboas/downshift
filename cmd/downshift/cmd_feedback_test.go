// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import "testing"

// feedback with no subcommand is a usage error, not a dump of recent routings.
func TestFeedbackWithoutSubcommandPrintsUsageAndExits2(t *testing.T) {
	if rc := runFeedback(nil); rc != 2 {
		t.Errorf("runFeedback(nil) = %d, want 2", rc)
	}
}
