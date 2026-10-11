// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry_test

import (
	"testing"

	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

func TestClassifyChildEndIsClosed(t *testing.T) {
	cases := []struct {
		event, toolStatus string
		interrupt         bool
		want              telemetry.ChildStatus
	}{
		{"PostToolUse", "completed", false, telemetry.ChildCompleted},
		{"PostToolUse", "async_launched", false, telemetry.ChildLaunched},
		{"PostToolUse", "error", false, telemetry.ChildUnknown},
		{"PostToolUseFailure", "", false, telemetry.ChildError},
		{"PostToolUseFailure", "", true, telemetry.ChildInterrupted},
		{"SubagentStop", "", false, telemetry.ChildStopped},
		{"PreToolUse", "", false, telemetry.ChildUnknown},
	}
	for _, c := range cases {
		if got := telemetry.ClassifyChildEnd(c.event, c.toolStatus, c.interrupt); got != c.want {
			t.Errorf("%s/%s/%v = %s, want %s", c.event, c.toolStatus, c.interrupt, got, c.want)
		}
	}
}
