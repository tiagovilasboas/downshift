// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package sensor_test

import (
	"bufio"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift/internal/compressor"
	"github.com/tiagovilasboas/downshift/internal/paths"
	"github.com/tiagovilasboas/downshift/internal/sensor"
)

func TestSensor_RecordToolOutput_AggregatesWithoutRawText(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(paths.EnvStateDir, tmp)

	store, err := sensor.DefaultStore()
	if err != nil {
		t.Fatalf("failed to create default store: %v", err)
	}

	testOutput := []byte("ok  github.com/foo/bar  0.1s  coverage: 80%\nok  github.com/foo/baz  0.2s  coverage: 90%\n")
	err = store.RecordToolOutput("test-session-123", "claude-code", "Bash", testOutput, false)
	if err != nil {
		t.Fatalf("record failed: %v", err)
	}

	summary, err := store.GetSummary()
	if err != nil {
		t.Fatalf("get summary failed: %v", err)
	}

	obs, ok := summary["test-session-123"]
	if !ok {
		t.Fatalf("expected observation for test-session-123")
	}

	if obs.ToolExecutions != 1 {
		t.Errorf("expected 1 execution, got %d", obs.ToolExecutions)
	}
	if obs.CompressionOpportunities != 1 {
		t.Errorf("expected 1 compression opportunity, got %d", obs.CompressionOpportunities)
	}
	if obs.FormatBreakdown[compressor.FormatGoTest] != 1 {
		t.Errorf("expected 1 go_test format counted")
	}
	if obs.PotentialReducedBytes <= 0 {
		t.Errorf("expected positive potential reduced bytes")
	}

	// Verify states
	if obs.InputTokens.State != sensor.StateUnavailable {
		t.Errorf("expected InputTokens to be StateUnavailable")
	}
}

func TestSummarizeCompaction(t *testing.T) {
	// Production session keys are sha256 hex digests. A shorter key fails closed.
	const sessionKey = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	obs := func(bytes, reduced int64) sensor.SessionObservation {
		return sensor.SessionObservation{
			ToolExecutions:        1,
			TotalOutputBytes:      bytes,
			PotentialReducedBytes: reduced,
		}
	}
	tests := []struct {
		name string
		in   map[string]sensor.SessionObservation
		want sensor.CompactionSummary
	}{
		{
			name: "empty",
			in:   nil,
			want: sensor.CompactionSummary{State: "unavailable", TokenState: "unavailable"},
		},
		{
			name: "bytes only",
			in:   map[string]sensor.SessionObservation{sessionKey: obs(16, 0)},
			want: sensor.CompactionSummary{
				Sessions:       1,
				ToolExecutions: 1,
				BytesBefore:    16,
				BytesReduced:   0,
				BytesAfter:     16,
				TokenState:     "unavailable",
				State:          "observed",
			},
		},
		{
			name: "reduced greater than before is clamped",
			in:   map[string]sensor.SessionObservation{sessionKey: obs(8, 100)},
			want: sensor.CompactionSummary{
				Sessions:       1,
				ToolExecutions: 1,
				BytesBefore:    8,
				BytesReduced:   8,
				BytesAfter:     0,
				TokenState:     "unavailable",
				SavingsPct:     100,
				State:          "observed",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sensor.SummarizeCompaction(tt.in)
			if got != tt.want {
				t.Errorf("SummarizeCompaction() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestRecordToolOutput_AppendsCompactionHistory(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(paths.EnvStateDir, tmp)

	store, err := sensor.DefaultStore()
	if err != nil {
		t.Fatalf("default store: %v", err)
	}

	const (
		sessionHash = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
		rawSession  = "session/raw-id"
		badTool     = "bad tool/../secret"
		sentinel    = "SECRET_TOOL_OUTPUT_SENTINEL"
	)
	compressible := []byte("ok  github.com/foo/bar  0.1s  coverage: 80%\nok  github.com/foo/baz  0.2s  coverage: 90%\n")
	plain := []byte("plain text " + sentinel)

	if err := store.RecordToolOutput(sessionHash, "claude-code", "Bash", compressible, false); err != nil {
		t.Fatalf("record compressible: %v", err)
	}
	if err := store.RecordToolOutput(rawSession, "claude-code", badTool, plain, true); err != nil {
		t.Fatalf("record plain: %v", err)
	}

	logPath := filepath.Join(tmp, "context-compactions.jsonl")
	f, err := os.Open(logPath)
	if err != nil {
		t.Fatalf("open history: %v", err)
	}
	defer f.Close()

	var lines []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		lines = append(lines, sc.Text())
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("scan history: %v", err)
	}
	if len(lines) != 2 {
		t.Fatalf("history lines = %d, want 2", len(lines))
	}

	raw := strings.Join(lines, "\n")
	for _, forbidden := range []string{sentinel, rawSession, badTool, "github.com/foo/bar", tmp} {
		if strings.Contains(raw, forbidden) {
			t.Errorf("history stored forbidden value %q", forbidden)
		}
	}

	compressed := compressor.Compress(compressible, compressor.ModeObserve)
	if !compressed.Applied {
		t.Fatal("fixture must be a recognized compressible output")
	}
	wantReduced := int64(compressed.OriginalBytes - compressed.ReducedBytes)

	var first sensor.CompactionRecord
	if err := json.Unmarshal([]byte(lines[0]), &first); err != nil {
		t.Fatalf("decode first line: %v", err)
	}
	ts, err := time.Parse(time.RFC3339Nano, first.Timestamp)
	if err != nil {
		t.Fatalf("timestamp: %v", err)
	}
	if ts.Location() != time.UTC {
		t.Errorf("timestamp location = %v, want UTC", ts.Location())
	}
	if first.Harness != "claude-code" || first.SessionHash != sessionHash || first.Tool != "Bash" {
		t.Errorf("first identity = harness %q session %q tool %q", first.Harness, first.SessionHash, first.Tool)
	}
	if first.OutputBytes != int64(len(compressible)) || first.ReducedBytes != wantReduced || !first.Applied || first.Error {
		t.Errorf("first sizes = %+v, want output %d reduced %d applied", first, len(compressible), wantReduced)
	}
	if !strings.Contains(lines[0], `"error":false`) {
		t.Errorf("first line omitted error field: %s", lines[0])
	}

	var second map[string]any
	if err := json.Unmarshal([]byte(lines[1]), &second); err != nil {
		t.Fatalf("decode second line: %v", err)
	}
	if _, ok := second["session_hash"]; ok {
		t.Errorf("invalid session hash was stored: %v", second["session_hash"])
	}
	if _, ok := second["tool"]; ok {
		t.Errorf("invalid tool name was stored: %v", second["tool"])
	}
	if second["harness"] != "claude-code" {
		t.Errorf("harness = %v", second["harness"])
	}
	if second["output_bytes"] != float64(len(plain)) || second["reduced_bytes"] != float64(0) || second["applied"] != false || second["error"] != true {
		t.Errorf("second line = %#v", second)
	}
}

func TestRecordToolOutput_FileModeAndLock(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(paths.EnvStateDir, tmp)
	store, err := sensor.DefaultStore()
	if err != nil {
		t.Fatal(err)
	}
	aggregate, err := paths.Join("context-sensor.json")
	if err != nil {
		t.Fatal(err)
	}
	lockPath := aggregate + ".lock"
	if err := os.WriteFile(lockPath, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestHoldStateLock$")
	cmd.Env = append(os.Environ(), "DOWNSHIFT_HOLD_LOCK=1", "DOWNSHIFT_LOCK_PATH="+lockPath)
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()
	scan := bufio.NewScanner(out)
	held := false
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if scan.Scan() && scan.Text() == "held" {
			held = true
			break
		}
	}
	if !held {
		t.Fatal("child did not acquire the lock")
	}

	done := make(chan error, 1)
	go func() {
		done <- store.RecordToolOutput("lock-session", "claude-code", "Bash", []byte("ok\n"), false)
	}()
	select {
	case err := <-done:
		t.Fatalf("write ran while another process held the state lock: %v", err)
	case <-time.After(80 * time.Millisecond):
	}
	_ = cmd.Process.Kill()
	_ = cmd.Wait()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("write did not finish after the other process exited")
	}

	info, err := os.Stat(aggregate)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("aggregate mode = %o, want 600", info.Mode().Perm())
	}
	history, err := paths.Join("context-compactions.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	hinfo, err := os.Stat(history)
	if err != nil {
		t.Fatal(err)
	}
	if hinfo.Mode().Perm() != 0o600 {
		t.Fatalf("history mode = %o, want 600", hinfo.Mode().Perm())
	}
}

func TestSummarizeCompactionLog_FailClosedAndCounts(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "context-compactions.jsonl")
	good := "{\"timestamp\":\"2026-10-09T00:00:00.000000000Z\",\"harness\":\"claude-code\",\"output_bytes\":10,\"reduced_bytes\":4,\"applied\":true,\"error\":false}\n"
	if err := os.WriteFile(path, []byte(good), 0o600); err != nil {
		t.Fatal(err)
	}
	rows, err := sensor.SummarizeCompactionLog(path)
	if err != nil {
		t.Fatal(err)
	}
	row, ok := rows["claude-code"]
	if !ok || row.TotalCommands != 1 || row.BytesReduced != 4 || row.TokenState != "unavailable" {
		t.Fatalf("summary = %+v", rows)
	}

	bad := good + "{\"timestamp\":\"2026-10-09T00:00:00.000000000Z\",\"harness\":\"claude-code\",\"output_bytes\":1,\"reduced_bytes\":0,\"applied\":false,\"error\":false,\"secret\":\"no\"}\n"
	if err := os.WriteFile(path, []byte(bad), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := sensor.SummarizeCompactionLog(path); err == nil {
		t.Fatal("an unknown field must reject the whole log")
	}
}
