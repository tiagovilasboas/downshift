// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package claudecode_test

import (
	"os"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/adapters/claudecode"
)

// Claude Code writes the effort a turn ran at on each assistant line of the
// subagent transcript. The usage record keeps it as final_reasoning_effort.
func TestSubagentStop_RecordsTranscriptEffort(t *testing.T) {
	log, small, frontier, res := stopSetup(t)
	seedLaunch(t, log, small, frontier, res)
	path := writeTranscript(t, small)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	withEffort := strings.ReplaceAll(string(data), `{"message":`, `{"effort":"medium","message":`)
	if withEffort == string(data) {
		t.Fatal("fixture has no assistant line to tag")
	}
	if err := os.WriteFile(path, []byte(withEffort), 0o600); err != nil {
		t.Fatal(err)
	}

	claudecode.HandleSubagentStop(stopPayload(path), "test", res)
	got := usageEvents(t)
	if len(got) != 1 || got[0].FinalEffort != "medium" {
		t.Fatalf("usage events = %+v, want one with final effort medium", got)
	}
}

// A transcript that names no effort leaves the field as it was.
func TestSubagentStop_NoTranscriptEffortLeavesFieldEmpty(t *testing.T) {
	log, small, frontier, res := stopSetup(t)
	seedLaunch(t, log, small, frontier, res)
	claudecode.HandleSubagentStop(stopPayload(writeTranscript(t, small)), "test", res)
	got := usageEvents(t)
	if len(got) != 1 || got[0].FinalEffort != "" {
		t.Fatalf("usage events = %+v, want one with no final effort", got)
	}
}
