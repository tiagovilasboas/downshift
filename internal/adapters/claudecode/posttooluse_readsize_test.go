// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package claudecode_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/adapters/claudecode"
	"github.com/tiagovilasboas/downshift-ai/internal/mainwork"
)

func TestPostToolUse_ReadResultSizeBucket(t *testing.T) {
	cases := []struct {
		name string
		resp any
		want string
		n    int
	}{
		{"absent", nil, "", 0},
		{"empty text", map[string]any{"content": ""}, "", 0},
		{"500 bytes", map[string]any{"content": strings.Repeat("a", 500)}, "<1K", 500},
		{"2 KB nested", map[string]any{"file": map[string]any{"content": strings.Repeat("a", 2048)}}, "1-4K", 2048},
		{"10 KB", map[string]any{"content": strings.Repeat("a", 10<<10)}, "4-20K", 10 << 10},
		{"20 KB", map[string]any{"content": strings.Repeat("a", 20<<10)}, "20-100K", 20 << 10},
		{"100 KB", map[string]any{"content": strings.Repeat("a", 100<<10)}, "20-100K", 100 << 10},
		{"plain string 200 KB", strings.Repeat("a", 200<<10), ">100K", 200 << 10},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := isolateDownshiftState(t)
			p := map[string]any{
				"session_id": "s", "tool_name": "Read",
				"tool_input": map[string]any{"file_path": "/secret/path.go"},
			}
			if c.resp != nil {
				p["tool_response"] = c.resp
			}
			raw, _ := json.Marshal(p)
			out, _, _ := claudecode.HandlePostToolUse(raw, "test", cat)
			if b, _ := json.Marshal(out); string(b) != "{}" {
				t.Errorf("hook emitted %s, want {}", b)
			}
			recs := mainWorkRecords(t, dir)
			if len(recs) != 1 || recs[0].ResultBytesBucket != c.want || recs[0].ResultBytes != c.n {
				t.Fatalf("records = %+v, want bucket %q and %d bytes", recs, c.want, c.n)
			}
			log, _ := os.ReadFile(filepath.Join(dir, mainwork.FileName))
			if strings.Contains(string(log), "secret") || strings.Contains(string(log), "aaaa") {
				t.Errorf("the log kept a path or content: %.200s", log)
			}
		})
	}
}

func TestPostToolUse_NonReadToolHasNoBucket(t *testing.T) {
	dir := isolateDownshiftState(t)
	raw, _ := json.Marshal(map[string]any{
		"session_id": "s", "tool_name": "Bash", "tool_response": map[string]any{"stdout": strings.Repeat("a", 5000)},
	})
	claudecode.HandlePostToolUse(raw, "test", cat)
	if recs := mainWorkRecords(t, dir); len(recs) != 1 || recs[0].ResultBytesBucket != "" {
		t.Fatalf("records = %+v, want one record without a bucket", recs)
	}
}
