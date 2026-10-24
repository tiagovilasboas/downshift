// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package cursor_test

import (
	"testing"

	"github.com/tiagovilasboas/downshift/internal/adapters/cursor"
)

func TestSessionIdentifier(t *testing.T) {
	const id = "bd265136-0000-4000-8000-000000000001"
	ev := cursor.Event{SessionID: id}
	if got := ev.SessionIdentifier(); got != id {
		t.Fatalf("SessionIdentifier() = %q, want %q", got, id)
	}
	var empty cursor.Event
	if got := empty.SessionIdentifier(); got != "" {
		t.Fatalf("empty event SessionIdentifier() = %q, want empty", got)
	}
}
