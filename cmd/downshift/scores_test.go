// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/scoreindex"
)

const scoresEpochFixture = `Model,Display name,eci,eci_ci_low,eci_ci_high,date
Claude Opus 5.5,Claude Opus 5.5,167.33,164.06,171.68,2026-09-22
Claude Sonnet 5.5,Claude Sonnet 5.5,165.03,161.68,169.06,2026-09-28
`

const scoresORFixture = `{"data":[
 {"id":"anthropic/claude-haiku-5.5","benchmarks":{"artificial_analysis":{"intelligence_index":43.4}}},
 {"id":"anthropic/claude-sonnet-5.5","benchmarks":{"artificial_analysis":{"intelligence_index":56.0}}},
 {"id":"anthropic/claude-opus-5.5","benchmarks":{"artificial_analysis":{"intelligence_index":57.6}}}
]}`

// scoresEnv isolates the state dir and writes a session file: claude-code has
// Haiku 5.5, so OpenRouter covers more than Epoch.
func scoresEnv(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("DOWNSHIFT_STATE_DIR", dir)
	t.Setenv("DOWNSHIFT_EVENT_LOG", filepath.Join(dir, "events.jsonl"))
	t.Setenv("DOWNSHIFT_SCORES", "")
	sm := filepath.Join(dir, "session-models.json")
	doc := `{"claude-code":["claude-haiku-5-5","claude-sonnet-5-5","claude-opus-5-5"],"codex":["gpt-6-luna"],"cursor":["composer-2.5"]}`
	if err := os.WriteFile(sm, []byte(doc), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DOWNSHIFT_SESSION_MODELS", sm)
	return dir
}

func scoresUpstream(t *testing.T, status int) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if status != 0 {
			http.Error(w, "down", status)
			return
		}
		if strings.HasSuffix(r.URL.Path, ".csv") {
			w.Header().Set("ETag", `"e1"`)
			_, _ = w.Write([]byte(scoresEpochFixture))
			return
		}
		_, _ = w.Write([]byte(scoresORFixture))
	}))
	t.Cleanup(srv.Close)
	m := scoreindex.DefaultIDMap()
	old := scoresSources
	scoresSources = func() []scoreindex.Source {
		return []scoreindex.Source{
			&scoreindex.Epoch{URL: srv.URL + "/eci.csv", Map: m, UserAgent: "t"},
			&scoreindex.OpenRouter{URL: srv.URL + "/models", Map: m, UserAgent: "t"},
		}
	}
	t.Cleanup(func() { scoresSources = old })
}

func runScores(args ...string) (int, string, string) {
	var out, errb bytes.Buffer
	rc := runModelsScores(args, &out, &errb)
	return rc, out.String(), errb.String()
}

func TestModelsScoresRefreshQuietThenStatus(t *testing.T) {
	dir := scoresEnv(t)
	scoresUpstream(t, 0)

	rc, out, errs := runScores("--refresh", "--quiet")
	if rc != 0 || out != "" || errs != "" {
		t.Fatalf("quiet refresh rc=%d out=%q err=%q", rc, out, errs)
	}
	if _, err := os.Stat(filepath.Join(dir, scoreCacheFile)); err != nil {
		t.Fatalf("cache not written under the contract name %s: %v", scoreCacheFile, err)
	}

	rc, out, errs = runScores("--status")
	if rc != 0 || errs != "" {
		t.Fatalf("status rc=%d err=%q", rc, errs)
	}
	for _, want := range []string{
		"selected source: openrouter-aa",
		"claude-code:", "claude-haiku-5-5", "43.40",
		"codex:", "gpt-6-luna", "uncovered", // OpenRouter fixture has no Luna
		"composer-2.5",
		"Artificial Analysis", "Epoch AI",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("status lacks %q:\n%s", want, out)
		}
	}
	// A bare invocation is the status view.
	if rc2, out2, _ := runScores(); rc2 != 0 || !strings.Contains(out2, "selected source") {
		t.Errorf("bare call: rc=%d out=%q", rc2, out2)
	}
}

func TestModelsScoresJSONStatus(t *testing.T) {
	scoresEnv(t)
	scoresUpstream(t, 0)
	if rc, _, _ := runScores("--refresh", "--quiet"); rc != 0 {
		t.Fatal("refresh failed")
	}
	rc, out, _ := runScores("--status", "--json")
	if rc != 0 {
		t.Fatalf("rc=%d", rc)
	}
	var st scoreindex.Status
	if err := json.Unmarshal([]byte(out), &st); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, out)
	}
	if st.SelectedSource != scoreindex.SourceOpenRouter || len(st.Sources) != 2 {
		t.Errorf("status = %+v", st)
	}
}

func TestModelsScoresRefreshPrintsSummary(t *testing.T) {
	scoresEnv(t)
	scoresUpstream(t, 0)
	rc, out, _ := runScores("--refresh")
	if rc != 0 || !strings.Contains(out, "epoch-ai") || !strings.Contains(out, "selected source openrouter-aa") {
		t.Fatalf("rc=%d out=%q", rc, out)
	}
}

func TestModelsScoresFailedRefreshKeepsCacheAndExitsNonZero(t *testing.T) {
	dir := scoresEnv(t)
	scoresUpstream(t, 0)
	if rc, _, _ := runScores("--refresh", "--quiet"); rc != 0 {
		t.Fatal("first refresh failed")
	}
	p := filepath.Join(dir, scoreCacheFile)
	before, _ := os.ReadFile(p)

	scoresUpstream(t, 500)
	rc, _, errs := runScores("--refresh", "--quiet")
	if rc != 1 || !strings.Contains(errs, "refresh failed") {
		t.Fatalf("rc=%d err=%q", rc, errs)
	}
	if after, _ := os.ReadFile(p); !bytes.Equal(before, after) {
		t.Error("failed refresh replaced the cache")
	}
}

func TestModelsScoresOffIsNoOp(t *testing.T) {
	dir := scoresEnv(t)
	scoresUpstream(t, 0)
	t.Setenv("DOWNSHIFT_SCORES", "off")
	rc, out, _ := runScores("--refresh")
	if rc != 0 || !strings.Contains(out, "disabled") {
		t.Fatalf("rc=%d out=%q", rc, out)
	}
	if _, err := os.Stat(filepath.Join(dir, scoreCacheFile)); !os.IsNotExist(err) {
		t.Error("off must not write the cache")
	}
	if rc, out, _ := runScores("--status"); rc != 0 || !strings.Contains(out, "off") {
		t.Errorf("status rc=%d out=%q", rc, out)
	}
}

func TestModelsScoresBadFlag(t *testing.T) {
	scoresEnv(t)
	if rc, _, _ := runScores("--nope"); rc != 2 {
		t.Errorf("rc=%d, want 2", rc)
	}
}

func TestRunModelsDispatchesScores(t *testing.T) {
	scoresEnv(t)
	var rc int
	out := captureStdout(func() { rc = runModels(cmdCat, []string{"scores", "--status"}) })
	if rc != 0 || !strings.Contains(out, "scores: missing") {
		t.Fatalf("rc=%d out=%q", rc, out)
	}
}

func TestScoreSessionIDsFollowsRegistry(t *testing.T) {
	scoresEnv(t)
	ids := scoreSessionIDs()
	for name := range harnessRegistry {
		if _, ok := ids[name]; !ok {
			t.Errorf("harness %s missing from the session map", name)
		}
	}
	if got := strings.Join(ids["claude-code"], ","); got != "claude-haiku-5-5,claude-sonnet-5-5,claude-opus-5-5" {
		t.Errorf("claude-code ids = %q", got)
	}
}
