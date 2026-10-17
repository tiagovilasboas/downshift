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

// Every name the Cursor and Codex parsers read maps the same way whatever the
// fallback; only a name outside the table takes the fallback.
func TestChildStatusFromText(t *testing.T) {
	synonyms := []struct {
		status string
		want   telemetry.ChildStatus
	}{
		{"completed", telemetry.ChildCompleted}, {"complete", telemetry.ChildCompleted}, {"success", telemetry.ChildCompleted},
		{"succeeded", telemetry.ChildCompleted}, {"done", telemetry.ChildCompleted}, {"COMPLETED", telemetry.ChildCompleted},
		{"error", telemetry.ChildError}, {"errored", telemetry.ChildError}, {"failed", telemetry.ChildError}, {"failure", telemetry.ChildError},
		{"Failed", telemetry.ChildError},
		{"interrupted", telemetry.ChildInterrupted}, {"aborted", telemetry.ChildInterrupted}, {"cancelled", telemetry.ChildInterrupted},
		{"canceled", telemetry.ChildInterrupted},
	}
	fallbacks := []telemetry.ChildStatus{telemetry.ChildStopped, telemetry.ChildLaunched}
	for _, fb := range fallbacks {
		for _, c := range synonyms {
			if got := telemetry.ChildStatusFromText(c.status, fb); got != c.want {
				t.Errorf("ChildStatusFromText(%q, %s) = %s, want %s", c.status, fb, got, c.want)
			}
		}
		for _, s := range []string{"", "weird", "async_launched", "running"} {
			if got := telemetry.ChildStatusFromText(s, fb); got != fb {
				t.Errorf("ChildStatusFromText(%q, %s) = %s, want the fallback", s, fb, got)
			}
		}
	}
}
