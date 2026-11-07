// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core

import "testing"

// A nested spawn may read its parent's model from the transcript only when the
// harness's transcript is not session-wide.
func TestTranscriptNamesSpawner(t *testing.T) {
	for _, tc := range []struct {
		name   string
		caps   HarnessCapabilities
		nested bool
		want   bool
	}{
		{"main spawn, session-wide transcript", HarnessCapabilities{SessionWideTranscript: true}, false, true},
		{"main spawn, per-agent transcript", HarnessCapabilities{}, false, true},
		{"nested spawn, session-wide transcript", HarnessCapabilities{SessionWideTranscript: true}, true, false},
		{"nested spawn, per-agent transcript", HarnessCapabilities{}, true, true},
	} {
		if got := tc.caps.TranscriptNamesSpawner(tc.nested); got != tc.want {
			t.Errorf("%s: TranscriptNamesSpawner(%v) = %v, want %v", tc.name, tc.nested, got, tc.want)
		}
	}
	if !ClaudeCodeCaps.SessionWideTranscript {
		t.Error("Claude Code hands every hook the main session's transcript_path (verified on 2.1.319)")
	}
}
