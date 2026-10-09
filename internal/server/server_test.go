// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLocalOnly(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // no events: handlers return empty data
	h := newHandler("")

	cases := []struct {
		name, path, host, origin string
		want                     int
	}{
		{"loopback ip", "/api/status", "127.0.0.1:7474", "", http.StatusOK},
		{"localhost", "/health", "localhost:7474", "", http.StatusOK},
		{"ipv6 loopback", "/health", "[::1]:7474", "", http.StatusOK},
		{"same-origin page", "/api/status", "127.0.0.1:7474", "http://127.0.0.1:7474", http.StatusOK},
		{"dns rebinding host", "/api/status", "evil.example:7474", "", http.StatusForbidden},
		{"foreign origin", "/api/status", "127.0.0.1:7474", "https://evil.example", http.StatusForbidden},
		{"foreign origin health", "/health", "localhost:7474", "https://evil.example", http.StatusForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			req.Host = tc.host
			if tc.origin != "" {
				req.Header.Set("Origin", tc.origin)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Errorf("status = %d, want %d", rec.Code, tc.want)
			}
			if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
				t.Errorf("unexpected Access-Control-Allow-Origin %q", got)
			}
		})
	}
}

func TestIsLocalHost(t *testing.T) {
	tests := []struct {
		name     string
		hostport string
		want     bool
	}{
		{"localhost", "localhost:7474", true},
		{"127.0.0.1", "127.0.0.1:7474", true},
		{"::1", "[::1]:7474", true},
		{"::1 no brackets", "::1", true},
		{"external host", "example.com:7474", false},
		{"external ip", "192.168.1.1:7474", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isLocalHost(tt.hostport); got != tt.want {
				t.Errorf("isLocalHost(%q) = %v, want %v", tt.hostport, got, tt.want)
			}
		})
	}
}

func TestHandleStatus_OK(t *testing.T) {
	tmpdir := t.TempDir()
	t.Setenv("HOME", tmpdir)
	downshiftDir := filepath.Join(tmpdir, ".harness-downshift")
	os.Mkdir(downshiftDir, 0o755)

	// Write a simple events.jsonl with one downshift event that was applied
	eventsFile := filepath.Join(downshiftDir, "events.jsonl")
	ts := time.Now().UTC().Format(time.RFC3339Nano)
	// outcome="rewrite_emitted" and corrections empty means it was applied
	event := fmt.Sprintf(`{"timestamp":"%s","harness":"claude-code","requested_model":"opus","final_model":"haiku","verdict":"DOWNSHIFT","complexity":"TRIVIAL","estimated_savings":0.8,"outcome":"rewrite_emitted","corrections":[]}`+"\n", ts)
	os.WriteFile(eventsFile, []byte(event), 0o644)

	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	rec := httptest.NewRecorder()
	handleStatus(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	body := rec.Body.Bytes()
	var resp statusResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		t.Fatalf("decode raw status: %v", err)
	}
	comp, _ := raw["compaction"].(map[string]any)
	for _, key := range []string{"tokens_before", "tokens_after", "tokens_reduced", "state"} {
		if _, ok := comp[key]; !ok {
			t.Errorf("compaction missing %q", key)
		}
	}

	if len(resp.Harnesses) == 0 {
		t.Error("expected harnesses to be populated")
	}
	if resp.Stats.Down != 1 {
		t.Errorf("Down count = %d, want 1", resp.Stats.Down)
	}
	if !resp.Stats.IsEstimate {
		t.Error("IsEstimate must be true")
	}
}

func TestHandleStatus_FilterStaleEvents(t *testing.T) {
	tmpdir := t.TempDir()
	t.Setenv("HOME", tmpdir)
	downshiftDir := filepath.Join(tmpdir, ".harness-downshift")
	os.Mkdir(downshiftDir, 0o755)

	// Write an event from 3 days ago (outside 24h window)
	eventsFile := filepath.Join(downshiftDir, "events.jsonl")
	oldTS := time.Now().UTC().AddDate(0, 0, -3).Format(time.RFC3339Nano)
	staleEvent := fmt.Sprintf(`{"timestamp":"%s","harness":"claude-code","verdict":"DOWNSHIFT"}`+"\n", oldTS)
	os.WriteFile(eventsFile, []byte(staleEvent), 0o644)

	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	rec := httptest.NewRecorder()
	handleStatus(rec, req)

	var resp statusResponse
	json.NewDecoder(rec.Body).Decode(&resp)

	if resp.Stats.Total > 0 {
		t.Errorf("stale events should be filtered out, got %d events", resp.Stats.Total)
	}
}

func TestFileSize(t *testing.T) {
	tmpdir := t.TempDir()

	// Non-existent file returns -1
	if got := fileSize(filepath.Join(tmpdir, "nonexistent")); got != -1 {
		t.Errorf("fileSize on missing file = %d, want -1", got)
	}

	// Existing file returns actual size
	testFile := filepath.Join(tmpdir, "test.txt")
	content := "hello"
	os.WriteFile(testFile, []byte(content), 0o644)
	if got := fileSize(testFile); got != int64(len(content)) {
		t.Errorf("fileSize = %d, want %d", got, len(content))
	}
}

func TestReadEvents_MalformedJSON(t *testing.T) {
	tmpdir := t.TempDir()
	t.Setenv("HOME", tmpdir)
	downshiftDir := filepath.Join(tmpdir, ".harness-downshift")
	os.Mkdir(downshiftDir, 0o755)

	// Write malformed JSON
	eventsFile := filepath.Join(downshiftDir, "events.jsonl")
	os.WriteFile(eventsFile, []byte("{not valid json}\n"), 0o644)

	// Should not panic; returns empty list
	events := readEvents()
	if len(events) != 0 {
		t.Errorf("malformed JSON should be skipped, got %d events", len(events))
	}
}

func TestReadAgents_Privacy(t *testing.T) {
	tmpdir := t.TempDir()
	t.Setenv("HOME", tmpdir)
	downshiftDir := filepath.Join(tmpdir, ".harness-downshift")
	os.Mkdir(downshiftDir, 0o755)

	// Write an agent entry with no task (new privacy-friendly format)
	agentsFile := filepath.Join(downshiftDir, "agents.jsonl")
	ts := time.Now().UTC().Format(time.RFC3339Nano)
	// Tool only, no task (privacy-compatible new format should be included)
	agentWithTool := fmt.Sprintf(`{"timestamp":"%s","tool":"Task"}`+"\n", ts)
	// Neither tool nor task (should be excluded)
	agentEmpty := fmt.Sprintf(`{"timestamp":"%s"}`+"\n", ts)

	content := agentWithTool + agentEmpty
	os.WriteFile(agentsFile, []byte(content), 0o644)

	agents := readAgents()
	if len(agents) != 1 {
		t.Errorf("got %d agents, want 1 (privacy filter)", len(agents))
	}
}

func TestHealth(t *testing.T) {
	h := newHandler("")
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Host = "localhost:7474" // localOnly requires loopback
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&resp)

	if ok, _ := resp["ok"].(bool); !ok {
		t.Error("health check should return {\"ok\":true}")
	}
}
