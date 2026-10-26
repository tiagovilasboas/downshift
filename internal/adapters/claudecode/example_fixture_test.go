// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package claudecode_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/adapters/claudecode"
	"github.com/tiagovilasboas/downshift/internal/core"
)

// The checked-in examples are the public PreToolUse shapes. Handle must accept
// them and downshift this trivial rename to the catalog native small name:
// the task fixture has no session list, so Claude Code infers one from the
// current model; the session fixture already carries that list.
func TestHandle_ExampleFixtures(t *testing.T) {
	for _, name := range []string{
		"claude-code-pretooluse-task.json",
		"claude-code-pretooluse-session.json",
	} {
		t.Run(name, func(t *testing.T) {
			var ev claudecode.Event
			if err := json.Unmarshal(readExample(t, name), &ev); err != nil {
				t.Fatalf("fixture is not a Claude Code event: %v", err)
			}
			out, note, _ := claudecode.Handle(withCatalogSession(ev), cat)
			m := decodeUpdated(t, out)
			want := nativeOf(core.TierSmall)
			if m == nil || m["model"] != want || note == "" {
				t.Fatalf("rewrite = model %v note %q, want native %s", modelOf(m), note, want)
			}
			if out.HookSpecificOutput == nil || out.HookSpecificOutput.PermissionDecision != "allow" {
				t.Fatal("rewrite must still allow the spawn")
			}
		})
	}
}

func readExample(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "..", "examples", name))
	if err != nil {
		t.Fatalf("read example fixture: %v", err)
	}
	return b
}

func modelOf(m map[string]any) any {
	if m == nil {
		return nil
	}
	return m["model"]
}
