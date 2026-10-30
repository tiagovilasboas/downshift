// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package claudecode

import "testing"

func TestClientIdentifierReadsEntrypointEnv(t *testing.T) {
	// Isolate state so the read never touches the developer's real dirs.
	t.Setenv("HOME", t.TempDir())
	t.Setenv("DOWNSHIFT_STATE_DIR", t.TempDir())

	t.Run("cli", func(t *testing.T) {
		t.Setenv(clientEntrypointEnv, "cli")
		if got := (Event{}).ClientIdentifier(); got != "cli" {
			t.Errorf("ClientIdentifier() = %q, want %q", got, "cli")
		}
	})

	t.Run("claude-desktop", func(t *testing.T) {
		t.Setenv(clientEntrypointEnv, "claude-desktop")
		if got := (Event{}).ClientIdentifier(); got != "claude-desktop" {
			t.Errorf("ClientIdentifier() = %q, want %q", got, "claude-desktop")
		}
	})

	t.Run("unset is empty", func(t *testing.T) {
		t.Setenv(clientEntrypointEnv, "")
		if got := (Event{}).ClientIdentifier(); got != "" {
			t.Errorf("ClientIdentifier() = %q, want empty", got)
		}
	})
}
