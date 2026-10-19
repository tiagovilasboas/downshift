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
)

// The sensor measured 598 Bash results and recorded 0 bytes for all of them:
// Claude Code's Bash tool_response carries {stdout, stderr}, not a `content`
// string, and the observer only read `content`. Each shape the hook can
// receive must give the length of the text the model reads, and the sensor
// must still store no text.
func TestPostToolUse_SensorMeasuresEveryToolResponseShape(t *testing.T) {
	const out = "ok  github.com/acme/a  0.1s\nok  github.com/acme/b  0.2s\n"
	tests := []struct {
		name string
		resp map[string]any
		want int64
	}{
		{"Bash stdout", map[string]any{"stdout": out, "stderr": ""}, int64(len(out))},
		{"Bash stdout and stderr", map[string]any{"stdout": "a\n", "stderr": "warn\n"}, int64(len("a\nwarn\n"))},
		{"stdout without a final newline gets one before stderr", map[string]any{"stdout": "a", "stderr": "b"}, int64(len("a\nb"))},
		{"stderr only", map[string]any{"stdout": "", "stderr": "boom\n"}, int64(len("boom\n"))},
		{"content string", map[string]any{"content": out}, int64(len(out))},
		{"content blocks", map[string]any{"content": []any{
			map[string]any{"type": "text", "text": "line one\n"}, map[string]any{"type": "image"}, map[string]any{"type": "text", "text": "line two\n"}}},
			int64(len("line one\nline two\n"))},
		{"output string", map[string]any{"output": out}, int64(len(out))},
		{"unknown shape records zero, as before", map[string]any{"result": 42}, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := isolateDownshiftState(t)
			writeContextConfig(t, dir, true)
			payload, err := json.Marshal(map[string]any{"session_id": "sess-bash-bytes", "tool_name": "Bash", "tool_response": tc.resp})
			if err != nil {
				t.Fatal(err)
			}
			claudecode.HandlePostToolUse(payload, "test", cat)
			raw, err := os.ReadFile(filepath.Join(dir, "context-sensor.json"))
			if err != nil {
				t.Fatal(err)
			}
			var sensor map[string]struct {
				Bytes int64 `json:"total_output_bytes"`
			}
			if err := json.Unmarshal(raw, &sensor); err != nil {
				t.Fatal(err)
			}
			var got int64
			for _, s := range sensor {
				got += s.Bytes
			}
			if got != tc.want {
				t.Errorf("recorded %d bytes, want %d", got, tc.want)
			}
			if strings.Contains(string(raw), "github.com/acme") || strings.Contains(string(raw), "boom") {
				t.Errorf("the sensor stored tool output: %s", raw)
			}
		})
	}
}

// A clean `go test` result in the real Bash shape is now seen as a compaction
// opportunity, which is what makes the sensor's potential saving non-zero.
func TestPostToolUse_BashStdoutShowsCompactionPotential(t *testing.T) {
	dir := isolateDownshiftState(t)
	writeContextConfig(t, dir, true)
	payload, _ := json.Marshal(map[string]any{
		"session_id": "sess-bash-potential", "tool_name": "Bash",
		"tool_response": map[string]any{"stdout": allOKToolOutput(), "stderr": "", "exit_code": 0},
	})
	claudecode.HandlePostToolUse(payload, "test", cat)
	rec := readCompaction(t, dir)
	if !rec.Applied || rec.ReducedBytes <= 0 || rec.OutputBytes != int64(len(allOKToolOutput())) {
		t.Fatalf("record = %+v, want an observed reduction on the Bash stdout shape", rec)
	}
}
