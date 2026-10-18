// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package cursor_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/adapters/cursor"
)

// cursor-pretooluse-task.json names a frontier model and carries no session
// list. Cursor does not infer a session, and these tests point the operator
// file at a missing path, so Handle fail-opens: allow, no note, no rewrite.
func TestHandle_ExampleFixtureFailsOpen(t *testing.T) {
	var ev cursor.Event
	if err := json.Unmarshal(readExample(t, "cursor-pretooluse-task.json"), &ev); err != nil {
		t.Fatalf("fixture is not a Cursor event: %v", err)
	}
	out, note, _ := cursor.Handle(ev, cat)
	if note != "" || len(out.UpdatedInput) != 0 || out.Permission != "allow" {
		t.Fatalf("fail-open = permission %q note %q updated %s", out.Permission, note, out.UpdatedInput)
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
