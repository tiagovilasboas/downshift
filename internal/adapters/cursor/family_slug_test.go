// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package cursor_test

import (
	"encoding/json"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/adapters/cursor"
)

// A Cursor picker slug such as grok-4.7 is not the catalog id (grok-4.7-xhigh)
// and is not an exact alias. When that catalog id is in the session, the spawn
// is on a session model and a confident trivial task must rewrite. When the
// resolved id is absent, the slug stays foreign and the spawn is unchanged.
func TestHandle_FamilySlugRewritesOnlyWhenResolvedIDIsInSession(t *testing.T) {
	const task = `{"task":"rename the userId variable to userIdentifier"}`
	inSession := []string{"composer-2.5-fast", "composer-2.5", "grok-4.7-xhigh"}
	ev := cursor.Event{ToolName: "Task", ModelID: "grok-4.7", SessionModels: &inSession, ToolInput: json.RawMessage(task)}
	out, note, _ := cursor.Handle(ev, cat)
	if note == "" || out.UpdatedInput == nil {
		t.Fatalf("session member via family slug must rewrite, note=%q", note)
	}
	if got := decodeUpdated(t, out)["model"]; got != "composer-2.5-fast" {
		t.Fatalf("model = %v, want composer-2.5-fast", got)
	}

	absent := []string{"composer-2.5-fast", "composer-2.5"}
	ev.SessionModels = &absent
	out, note, _ = cursor.Handle(ev, cat)
	if note != "" || out.UpdatedInput != nil {
		t.Fatalf("family slug outside the session must stay, note=%q updated=%s", note, out.UpdatedInput)
	}
}
