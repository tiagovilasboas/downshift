// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package claudecode_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/adapters/claudecode"
	"github.com/tiagovilasboas/downshift/internal/paths"
)

const toolEvidence = "ok  github.com/acme/a  0.1s\nok  github.com/acme/b  0.2s\nKEEP_TOOL_EVIDENCE_LINE\n"

func TestPostToolUse_CompressExitFollowsNativeFlag(t *testing.T) {
	t.Run("off", func(t *testing.T) {
		dir := isolateDownshiftState(t)
		writeContextConfig(t, dir, false)
		out := postTool(t, allOKToolOutput(), 0)
		assertToolEvidenceKept(t, out)
		if _, err := os.Stat(filepath.Join(dir, "context-compactions.jsonl")); !os.IsNotExist(err) {
			t.Fatal("CompressExit was recorded while the native provider flag was off")
		}
	})

	t.Run("on observes exit 0 without rewriting the tool", func(t *testing.T) {
		dir := isolateDownshiftState(t)
		writeContextConfig(t, dir, true)
		out := postTool(t, allOKToolOutput(), 0)
		assertToolEvidenceKept(t, out)
		rec := readCompaction(t, dir)
		if rec.Harness != "claude-code" || !rec.Applied || rec.ReducedBytes <= 0 || rec.OutputBytes != int64(len(allOKToolOutput())) {
			t.Fatalf("record = %+v, want an observe-mode reduction for exit 0", rec)
		}
		sensorRaw, err := os.ReadFile(filepath.Join(dir, "context-sensor.json"))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(sensorRaw), "github.com/acme/a") {
			t.Fatal("sensor stored tool output")
		}
	})

	t.Run("on keeps a non-zero exit uncompressed", func(t *testing.T) {
		dir := isolateDownshiftState(t)
		writeContextConfig(t, dir, true)
		out := postTool(t, allOKToolOutput(), 2)
		assertToolEvidenceKept(t, out)
		rec := readCompaction(t, dir)
		if rec.Applied || rec.ReducedBytes != 0 || rec.OutputBytes != int64(len(allOKToolOutput())) {
			t.Fatalf("record = %+v, want the real exit code to preserve bytes", rec)
		}
	})

	t.Run("on does not drop a line the hook cannot rewrite", func(t *testing.T) {
		dir := isolateDownshiftState(t)
		writeContextConfig(t, dir, true)
		out := postTool(t, toolEvidence, 0)
		assertToolEvidenceKept(t, out)
		encoded, err := json.Marshal(out)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(encoded), "Go test: PASS") {
			t.Fatalf("hook replaced tool evidence with a summary: %s", encoded)
		}
		if strings.Contains(string(encoded), "content") && !strings.Contains(string(encoded), "KEEP_TOOL_EVIDENCE_LINE") {
			t.Fatalf("rewritten tool result dropped evidence: %s", encoded)
		}
	})
}

func isolateDownshiftState(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv(paths.EnvStateDir, dir)
	t.Setenv("DOWNSHIFT_EVENT_LOG", filepath.Join(dir, "events.jsonl"))
	return dir
}

func writeContextConfig(t *testing.T, dir string, enabled bool) {
	t.Helper()
	provider := ""
	if enabled {
		provider = "native"
	}
	body, err := json.Marshal(map[string]any{"enabled": enabled, "provider": provider})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "context-optimization.json"), append(body, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
}

func allOKToolOutput() string {
	return "ok  github.com/acme/a  0.1s\nok  github.com/acme/b  0.2s\n"
}

func postTool(t *testing.T, content string, exitCode int) claudecode.Output {
	t.Helper()
	payload, err := json.Marshal(map[string]any{
		"session_id": "sess-qa-compress",
		"tool_name":  "Bash",
		"tool_response": map[string]any{
			"content":   content,
			"exit_code": exitCode,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	out, _, _ := claudecode.HandlePostToolUse(payload, "test", cat)
	return out
}

func assertToolEvidenceKept(t *testing.T, out claudecode.Output) {
	t.Helper()
	encoded, err := json.Marshal(out)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "Go test: PASS") {
		t.Fatalf("hook rewrote the tool result: %s", encoded)
	}
}

type compactionLine struct {
	Harness      string `json:"harness"`
	OutputBytes  int64  `json:"output_bytes"`
	ReducedBytes int64  `json:"reduced_bytes"`
	Applied      bool   `json:"applied"`
}

func readCompaction(t *testing.T, dir string) compactionLine {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, "context-compactions.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	var rec compactionLine
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(raw))), &rec); err != nil {
		t.Fatal(err)
	}
	return rec
}
