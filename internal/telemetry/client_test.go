// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// rawLine returns the single JSONL line written to path.
func rawLine(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(data))
}

func TestClientOrDrop(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{name: "cli kept", in: "cli", want: "cli"},
		{name: "claude-desktop kept", in: "claude-desktop", want: "claude-desktop"},
		{name: "dotted and underscored kept", in: "claude_code.2-1", want: "claude_code.2-1"},
		{name: "empty dropped", in: "", want: ""},
		{name: "uppercase dropped", in: "CLI", want: ""},
		{name: "space dropped", in: "claude desktop", want: ""},
		{name: "slash dropped", in: "claude/desktop", want: ""},
		{name: "too long dropped", in: strings.Repeat("a", 33), want: ""},
		{name: "at the cap kept", in: strings.Repeat("a", 32), want: strings.Repeat("a", 32)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ClientOrDrop(tc.in); got != tc.want {
				t.Errorf("ClientOrDrop(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// The field rides the event through Record to the log, and stays absent (no
// "client" key) when empty thanks to omitempty.
func TestEventCarriesClient(t *testing.T) {
	t.Run("present", func(t *testing.T) {
		log := filepath.Join(t.TempDir(), "events.jsonl")
		t.Setenv("DOWNSHIFT_STATE_DIR", t.TempDir())
		t.Setenv("DOWNSHIFT_EVENT_LOG", log)

		Record(Event{CorrelationID: "c", Harness: "claude-code", Outcome: OutcomeAllow, Client: "claude-desktop"})
		events, err := ReadEventsFrom(log)
		if err != nil || len(events) != 1 {
			t.Fatalf("ReadEventsFrom = %d events, %v", len(events), err)
		}
		if events[0].Client != "claude-desktop" {
			t.Errorf("recorded client = %q, want %q", events[0].Client, "claude-desktop")
		}
		if line := rawLine(t, log); !strings.Contains(line, `"client":"claude-desktop"`) {
			t.Errorf("line missing client key: %s", line)
		}
	})

	t.Run("absent when empty", func(t *testing.T) {
		log := filepath.Join(t.TempDir(), "events.jsonl")
		t.Setenv("DOWNSHIFT_STATE_DIR", t.TempDir())
		t.Setenv("DOWNSHIFT_EVENT_LOG", log)

		Record(Event{CorrelationID: "c", Harness: "claude-code", Outcome: OutcomeAllow})
		if line := rawLine(t, log); strings.Contains(line, `"client"`) {
			t.Errorf("empty client must be omitted, got: %s", line)
		}
	})
}
