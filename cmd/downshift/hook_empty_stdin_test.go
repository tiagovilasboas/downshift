// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/adapters/cursor"
	"github.com/tiagovilasboas/downshift/internal/core"
)

// Cursor has been reported to run preToolUse hooks with empty stdin on remote
// workspaces. The shared runner must answer fail-open and exit 0 for empty or
// whitespace-only input, never block the spawn.
func TestRunHookAdapter_EmptyStdinFailsOpen(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("DOWNSHIFT_EVENT_LOG", t.TempDir()+"/events.jsonl")
	for _, in := range []string{"", "  \n"} {
		failedOpen := false
		rc := runHookAdapter(
			"cursor",
			strings.NewReader(in),
			func(b []byte) (cursor.Event, error) { var e cursor.Event; return e, json.Unmarshal(b, &e) },
			func(e cursor.Event) (any, string, core.Decision) { return cursor.Handle(e, cmdCat) },
			rewriteWire(func() { failedOpen = true }),
		)
		if rc != 0 {
			t.Errorf("input %q: rc = %d, want 0", in, rc)
		}
		if !failedOpen {
			t.Errorf("input %q: fail-open response not emitted", in)
		}
	}
}
