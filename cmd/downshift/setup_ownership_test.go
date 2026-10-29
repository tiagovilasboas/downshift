// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"strings"
	"testing"
)

const kiroSteeringPath = ".kiro/crew/workspace/.kiro/steering/downshift-compact.md"

// A dedicated steering file that a person edited after setup is theirs now:
// undo must leave it, not restore over it.
func TestSetupUndoLeavesAnEditedSteeringFile(t *testing.T) {
	e := newSetupEnv(t, ".kiro")
	if code, out, _ := e.run("--yes", "kirocrew"); code != 0 {
		t.Fatalf("setup exit %d\n%s", code, out)
	}
	path := e.path(kiroSteeringPath)
	if err := os.WriteFile(path, []byte("my own notes\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	code, out, _ := e.run("--undo", "--yes")
	if code != 0 || !strings.Contains(out, "edited after setup") {
		t.Fatalf("undo exit %d, want 0 with a skip note\n%s", code, out)
	}
	if got, _ := os.ReadFile(path); string(got) != "my own notes\n" {
		t.Fatalf("undo overwrote the user's file: %q", got)
	}
}

// A file setup did not write is not replaced without a person moving it aside.
func TestSteeringRefusesAnUnownedFile(t *testing.T) {
	e := newSetupEnv(t, ".kiro")
	path := e.path(kiroSteeringPath)
	if err := os.MkdirAll(e.path(".kiro/crew/workspace/.kiro/steering"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("my own notes\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	code, _, errOut := e.run("--yes", "kirocrew")
	if code == 0 || !strings.Contains(errOut, "not written by downshift setup") {
		t.Fatalf("setup exit %d, want a refusal\n%s", code, errOut)
	}
	if got, _ := os.ReadFile(path); string(got) != "my own notes\n" {
		t.Fatalf("setup overwrote an unowned file: %q", got)
	}
}

// The text an earlier setup wrote, before the marker existed, is still ours.
func TestSteeringAdoptsItsLegacyFile(t *testing.T) {
	e := newSetupEnv(t, ".kiro")
	path := e.path(kiroSteeringPath)
	if err := os.MkdirAll(e.path(".kiro/crew/workspace/.kiro/steering"), 0o755); err != nil {
		t.Fatal(err)
	}
	legacy := "---\ninclusion: always\n---\n\n" + compactSteering
	if err := os.WriteFile(path, []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}
	if code, out, _ := e.run("--yes", "kirocrew"); code != 0 {
		t.Fatalf("setup exit %d\n%s", code, out)
	}
	if got, _ := os.ReadFile(path); !setupOwns(got) {
		t.Fatalf("the adopted file has no ownership marker")
	}
}
