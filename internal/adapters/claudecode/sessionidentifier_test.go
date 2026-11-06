// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package claudecode

import "testing"

func TestSessionIdentifier(t *testing.T) {
	const id = "0b1c2d3e-4f50-4a6b-8c7d-9e0f1a2b3c4d"
	cases := []struct {
		name string
		ev   Event
		want string
	}{
		{"session_id wins", Event{SessionID: "abc", TranscriptPath: "/p/" + id + ".jsonl"}, "abc"},
		{"transcript only", Event{TranscriptPath: "/home/u/.claude/projects/x/" + id + ".jsonl"}, id},
		{"neither", Event{}, ""},
		{"non-uuid file name", Event{TranscriptPath: "/p/transcript.jsonl"}, ""},
		{"uuid-shaped with bad chars", Event{TranscriptPath: "/p/0b1c2d3e-4f50-4a6b-8c7d-9e0f1a2b3czz.jsonl"}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.ev.SessionIdentifier(); got != tc.want {
				t.Fatalf("SessionIdentifier() = %q, want %q", got, tc.want)
			}
		})
	}
}
