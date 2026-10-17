// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package cursor_test

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/adapters/cursor"
)

func TestHandle_ParentModelIDIsGuessedBaseline(t *testing.T) {
	t.Setenv("DOWNSHIFT_CURSOR_NATIVE_FILE", "")
	t.Setenv("DOWNSHIFT_QUOTA_FILE", filepath.Join(t.TempDir(), "no-quota.json"))
	cur := "claude-4.5-sonnet-thinking"
	session := []string{"composer-2.5-fast", cur, "composer-2.5"}
	ev := cursor.Event{ToolName: "Task", SessionModels: &session, ModelID: cur,
		ToolInput: json.RawMessage(`{"task": "implement the feature to export sales as CSV"}`)}
	if _, _, d := cursor.Handle(ev, cat); !d.Baseline.Guessed {
		t.Fatalf("parent model_id baseline must be marked guessed: %+v", d)
	}
	ev.ToolInput = json.RawMessage(`{"task": "implement the feature to export sales as CSV", "model": "` + cur + `"}`)
	if _, _, d := cursor.Handle(ev, cat); d.Baseline.Guessed {
		t.Fatalf("a model named by the call is not a guess: %+v", d)
	}
}
