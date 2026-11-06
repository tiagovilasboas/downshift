// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"strings"
	"testing"
)

// A hook runs honor, usage and end, so a wrong argument count must not exit 2:
// Claude Code reads exit 2 from a SubagentStop hook as "keep the subagent
// running" and from PostToolUse as feedback for the model.
func TestObserverUsageErrorExitsZero(t *testing.T) {
	for _, cmd := range []string{"honor", "usage", "end"} {
		for _, args := range [][]string{nil, {"claude-code", "extra"}} {
			var errW bytes.Buffer
			if rc := runObserver(cmd, args, strings.NewReader(""), &errW, cmdCat); rc != 0 {
				t.Errorf("%s %q: exit %d, want 0", cmd, args, rc)
			}
			if want := "usage: downshift " + cmd + " <harness>\n"; errW.String() != want {
				t.Errorf("%s %q: stderr %q, want %q", cmd, args, errW.String(), want)
			}
		}
	}
}
