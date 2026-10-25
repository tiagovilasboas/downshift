// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package discovery

import (
	"os"
	"path/filepath"
	"testing"
)

// writeState creates subagents/<dir>/state.json. Fields left empty are omitted
// so a test can exercise the resolved/requested/parent precedence in turn.
func writeState(t *testing.T, crew, dir, sessionID, parent, resolved, requested string) {
	t.Helper()
	d := filepath.Join(crew, "subagents", dir)
	if err := os.MkdirAll(d, 0o700); err != nil {
		t.Fatal(err)
	}
	body := `{"session_id":"` + sessionID + `"`
	if parent != "" {
		body += `,"parent_session":"` + parent + `"`
	}
	if resolved != "" {
		body += `,"resolved_model":"` + resolved + `"`
	}
	if requested != "" {
		body += `,"requested_model":"` + requested + `"`
	}
	body += `}`
	if err := os.WriteFile(filepath.Join(d, "state.json"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

// writeSessionLog writes sessions/<name>.jsonl from raw lines (one JSON object
// per line, as KiroCrew stores them).
func writeSessionLog(t *testing.T, crew, name string, lines ...string) {
	t.Helper()
	d := filepath.Join(crew, "sessions")
	if err := os.MkdirAll(d, 0o700); err != nil {
		t.Fatal(err)
	}
	content := ""
	for _, l := range lines {
		content += l + "\n"
	}
	if err := os.WriteFile(filepath.Join(d, name+".jsonl"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func asst(model string) string {
	return `{"role":"assistant","meta":{"turn_stats":{"model":"` + model + `","elapsed_ms":10}}}`
}

func setupCrew(t *testing.T) string {
	t.Helper()
	crew := t.TempDir()
	t.Setenv(EnvKiroCrewDir, crew)
	ResetKiroModelCache()
	return crew
}

func TestInferKiroCrewModel_ResolvedModelWins(t *testing.T) {
	crew := setupCrew(t)
	// A parent log with a different model must be ignored when state.json has one.
	writeState(t, crew, "sub1", "sess-1", "dashboard:chat-7-1", "glm-5", "auto")
	writeSessionLog(t, crew, "dashboard_chat-7-1", asst("claude-opus-4.5"))

	got, err := InferKiroCrewModel("sess-1")
	if err != nil || got != "glm-5" {
		t.Fatalf("got %q, %v; want glm-5", got, err)
	}
}

func TestInferKiroCrewModel_RequestedFallback(t *testing.T) {
	crew := setupCrew(t)
	writeState(t, crew, "sub1", "sess-2", "", "", "claude-haiku-4.5")

	got, err := InferKiroCrewModel("sess-2")
	if err != nil || got != "claude-haiku-4.5" {
		t.Fatalf("got %q, %v; want claude-haiku-4.5", got, err)
	}
}

func TestInferKiroCrewModel_FromParentSessionLog(t *testing.T) {
	crew := setupCrew(t)
	// resolved/requested are routing modes only -> must walk the parent log.
	writeState(t, crew, "sub1", "sess-3", "dashboard:chat-20-2", "", "auto")
	writeSessionLog(t, crew, "dashboard_chat-20-2",
		`{"role":"user","content":"hi"}`,
		asst("claude-sonnet-4.5"),
		`{"role":"user","content":"more"}`,
		asst("claude-sonnet-4.6"), // last assistant turn wins
	)

	got, err := InferKiroCrewModel("sess-3")
	if err != nil || got != "claude-sonnet-4.6" {
		t.Fatalf("got %q, %v; want claude-sonnet-4.6", got, err)
	}
}

func TestInferKiroCrewModel_UnknownWhenNoState(t *testing.T) {
	setupCrew(t)
	got, err := InferKiroCrewModel("no-such-session")
	if got != KiroModelUnknown || err == nil {
		t.Fatalf("got %q, %v; want unknown + error", got, err)
	}
}

func TestInferKiroCrewModel_UnknownWhenOnlyRoutingModes(t *testing.T) {
	crew := setupCrew(t)
	writeState(t, crew, "sub1", "sess-4", "dashboard:chat-1-9", "auto", "opusplan")
	writeSessionLog(t, crew, "dashboard_chat-1-9", asst("auto"), asst("opusplan"))

	got, err := InferKiroCrewModel("sess-4")
	if got != KiroModelUnknown || err == nil {
		t.Fatalf("got %q, %v; want unknown + error", got, err)
	}
}

func TestInferKiroCrewModel_EmptySession(t *testing.T) {
	setupCrew(t)
	if got, err := InferKiroCrewModel("  "); got != KiroModelUnknown || err == nil {
		t.Fatalf("got %q, %v; want unknown + error", got, err)
	}
}

func TestInferKiroCrewModel_Cached(t *testing.T) {
	crew := setupCrew(t)
	writeState(t, crew, "sub1", "sess-5", "", "", "glm-5")
	if got, _ := InferKiroCrewModel("sess-5"); got != "glm-5" {
		t.Fatalf("first read got %q", got)
	}
	// Remove the state file; a cached hit must still answer glm-5.
	if err := os.RemoveAll(filepath.Join(crew, "subagents")); err != nil {
		t.Fatal(err)
	}
	if got, err := InferKiroCrewModel("sess-5"); err != nil || got != "glm-5" {
		t.Fatalf("cached read got %q, %v; want glm-5", got, err)
	}
}

func TestSessionLogName(t *testing.T) {
	cases := map[string]string{
		"dashboard:chat-7-1790861092": "dashboard_chat-7-1790861092.jsonl",
		"cron_0f219623":               "cron_0f219623.jsonl",
		"a:b:c":                       "a_b:c.jsonl", // only the first ':' is replaced
	}
	for in, want := range cases {
		if got := sessionLogName(in); got != want {
			t.Errorf("sessionLogName(%q) = %q; want %q", in, got, want)
		}
	}
}

func TestModelFromSessionLog_RejectsTraversal(t *testing.T) {
	crew := setupCrew(t)
	// A parent reference that would escape the sessions dir resolves to "".
	if got := modelFromSessionLog(crew, "../../etc/passwd"); got != "" {
		t.Fatalf("traversal got %q; want empty", got)
	}
}

func TestCleanModel(t *testing.T) {
	cases := map[string]string{
		"claude-opus-4.5": "claude-opus-4.5",
		"  glm-5  ":       "glm-5",
		"auto":            "",
		"opusplan":        "",
		"":                "",
		"bad model id":    "", // space -> rejected by id pattern
	}
	for in, want := range cases {
		if got := cleanModel(in); got != want {
			t.Errorf("cleanModel(%q) = %q; want %q", in, got, want)
		}
	}
}

func TestInferKiroCrewMetrics(t *testing.T) {
	// Create a temp CLI sessions directory
	tmp := t.TempDir()
	cliDir := filepath.Join(tmp, ".kiro", "sessions", "cli")
	if err := os.MkdirAll(cliDir, 0o700); err != nil {
		t.Fatal(err)
	}

	// Mock home directory
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmp)
	defer os.Setenv("HOME", oldHome)

	sessionID := "12345678-1234-1234-1234-123456789abc"
	sessionJSON := `{
		"session_state": {
			"rts_model_state": {
				"model_info": {
					"model_id": "claude-sonnet-4"
				}
			},
			"conversation_metadata": {
				"user_turn_metadatas": [
					{"context_used": 50000, "context_window": 200000},
					{"context_used": 96745, "context_window": 200000}
				]
			}
		}
	}`
	if err := os.WriteFile(filepath.Join(cliDir, sessionID+".json"), []byte(sessionJSON), 0o600); err != nil {
		t.Fatal(err)
	}

	metrics, err := InferKiroCrewMetrics(sessionID)
	if err != nil {
		t.Fatalf("InferKiroCrewMetrics failed: %v", err)
	}

	if metrics.Model != "claude-sonnet-4" {
		t.Errorf("Model = %q; want claude-sonnet-4", metrics.Model)
	}
	if metrics.ContextUsed != 96745 {
		t.Errorf("ContextUsed = %d; want 96745", metrics.ContextUsed)
	}
	if metrics.ContextWindow != 200000 {
		t.Errorf("ContextWindow = %d; want 200000", metrics.ContextWindow)
	}
}

func TestKiroSessionMetrics_ContextUtilization(t *testing.T) {
	cases := []struct {
		name    string
		metrics KiroSessionMetrics
		want    float64
	}{
		{
			name:    "normal",
			metrics: KiroSessionMetrics{ContextUsed: 100000, ContextWindow: 200000},
			want:    50.0,
		},
		{
			name:    "zero window",
			metrics: KiroSessionMetrics{ContextUsed: 100000, ContextWindow: 0},
			want:    0.0,
		},
		{
			name:    "full",
			metrics: KiroSessionMetrics{ContextUsed: 200000, ContextWindow: 200000},
			want:    100.0,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.metrics.ContextUtilization()
			if got != tc.want {
				t.Errorf("ContextUtilization() = %f; want %f", got, tc.want)
			}
		})
	}
}
