// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package cursor_test

import (
	"encoding/json"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/adapters/cursor"
)

func TestComposerCache_RecallAfterTouch(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cursor.TouchFromStop("sess-1", "", "composer-2.5-fast")
	ev := cursor.Event{
		SessionID: "sess-1",
		ToolName:  "Task",
		ToolInput: json.RawMessage(`{"prompt":"rename userId"}`),
	}
	if got := ev.RequestedModel(); got != "composer-2.5-fast" {
		t.Fatalf("RequestedModel() = %q, want composer cache", got)
	}
}
