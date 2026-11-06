// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package claudecode_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/adapters/claudecode"
	"github.com/tiagovilasboas/downshift/internal/mainwork"
)

func mainWorkRecords(t *testing.T, dir string) []mainwork.Record {
	t.Helper()
	recs, err := mainwork.Read(filepath.Join(dir, mainwork.FileName))
	if err != nil {
		t.Fatal(err)
	}
	return recs
}

func TestPostToolUse_ReadIsRecordedAsMainWork(t *testing.T) {
	dir := isolateDownshiftState(t)
	payload, _ := json.Marshal(map[string]any{
		"session_id": "sess-read", "tool_name": "Read",
		"tool_input": map[string]any{"file_path": "/secret/path.go"}, "tool_response": map[string]any{"content": "x"},
	})
	claudecode.HandlePostToolUse(payload, "test", cat)
	recs := mainWorkRecords(t, dir)
	if len(recs) != 1 || recs[0].Tool != "Read" || recs[0].Category != "read" || recs[0].Harness != "claude-code" {
		t.Fatalf("records = %+v, want one Read/read record", recs)
	}
	raw, _ := os.ReadFile(filepath.Join(dir, mainwork.FileName))
	if strings.Contains(string(raw), "secret") {
		t.Errorf("the log kept a path: %s", raw)
	}
}

func TestPostToolUse_SubagentReadIsNotMainWork(t *testing.T) {
	dir := isolateDownshiftState(t)
	payload, _ := json.Marshal(map[string]any{"session_id": "s", "agent_id": "agent-1", "tool_name": "Grep"})
	claudecode.HandlePostToolUse(payload, "test", cat)
	if recs := mainWorkRecords(t, dir); len(recs) != 0 {
		t.Fatalf("a subagent call was recorded: %+v", recs)
	}
}

// Read and search results reach only the work count, never the context sensor.
func TestPostToolUse_ReadDoesNotReachTheContextSensor(t *testing.T) {
	dir := isolateDownshiftState(t)
	writeContextConfig(t, dir, true)
	payload, _ := json.Marshal(map[string]any{
		"session_id": "sess-sensor-read", "tool_name": "Glob", "tool_response": map[string]any{"content": "a\nb\n"},
	})
	claudecode.HandlePostToolUse(payload, "test", cat)
	if _, err := os.Stat(filepath.Join(dir, "context-sensor.json")); err == nil {
		t.Error("the context sensor observed a Glob call")
	}
}

func TestPostToolUse_SubagentTranscriptPathIsNotMainWork(t *testing.T) {
	dir := isolateDownshiftState(t)
	payload, _ := json.Marshal(map[string]any{"session_id": "s", "tool_name": "Read", "transcript_path": "/h/.claude/projects/p/s/subagents/agent-1.jsonl"})
	claudecode.HandlePostToolUse(payload, "test", cat)
	if recs := mainWorkRecords(t, dir); len(recs) != 0 {
		t.Fatalf("a subagent transcript call was recorded: %+v", recs)
	}
}
