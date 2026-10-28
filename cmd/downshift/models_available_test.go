// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift/internal/paths"
)

// availableState isolates the state dir and the discovery switches, which a
// developer shell may set (DOWNSHIFT_DISCOVERY=off).
func availableState(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
	t.Setenv(paths.EnvStateDir, dir)
	for _, k := range []string{"DOWNSHIFT_DISCOVERED", "DOWNSHIFT_SESSION_MODELS", "DOWNSHIFT_DISCOVERY", "DOWNSHIFT_DISCOVERY_TTL"} {
		t.Setenv(k, "")
	}
	return dir
}

func runAvailable(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	code := runModelsAvailable(cmdCat, args, &out, &errb)
	return code, out.String(), errb.String()
}

func TestModelsAvailableTextAndJSON(t *testing.T) {
	dir := availableState(t)
	cache := `{"version":1,"harnesses":{"cursor":{"fetched_at":"` + time.Now().UTC().Format(time.RFC3339) + `",` +
		`"ordered":["alpha-1","beta-2"],"unranked":["gamma-3"],` +
		`"model_metadata":[{"id":"alpha-1","tier":"small","efforts":["low"]}]}}}`
	writeFile(t, filepath.Join(dir, "discovered.json"), cache)
	writeFile(t, filepath.Join(dir, "session-models.json"), `{"codex":["file-1"]}`)

	code, out, _ := runAvailable(t, "--harness", "cursor")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	for _, want := range []string{"discovery cache written", "alpha-1", "small", "discovery", "beta-2", "gamma-3", "[untiered"} {
		if !strings.Contains(out, want) {
			t.Errorf("text output misses %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, dir) {
		t.Errorf("output leaks the state dir path:\n%s", out)
	}

	code, out, _ = runAvailable(t, "--harness=cursor", "--json")
	var got struct {
		Harness  string `json:"harness"`
		Fallback bool   `json:"fallback"`
		Written  string `json:"cache_written_at"`
		Models   []struct {
			ID       string `json:"id"`
			Tier     string `json:"tier"`
			Source   string `json:"source"`
			Untiered bool   `json:"untiered"`
		} `json:"models"`
	}
	if code != 0 || json.Unmarshal([]byte(out), &got) != nil {
		t.Fatalf("exit %d, json %q", code, out)
	}
	if got.Harness != "cursor" || got.Fallback || got.Written == "" || len(got.Models) != 3 {
		t.Fatalf("json = %+v", got)
	}
	if got.Models[0].Tier != "small" || got.Models[0].Untiered || !got.Models[1].Untiered || got.Models[2].ID != "gamma-3" {
		t.Errorf("models = %+v", got.Models)
	}

	code, out, _ = runAvailable(t, "--harness", "codex", "--json")
	if code != 0 || !strings.Contains(out, `"source": "file"`) || strings.Contains(out, "cache_written_at") {
		t.Errorf("file source: exit %d\n%s", code, out)
	}
}

func TestModelsAvailableNoListIsPlainAndExitZero(t *testing.T) {
	availableState(t)
	code, out, _ := runAvailable(t, "--harness", "cursor")
	want := "no detected list for cursor; run downshift models discover --harness cursor"
	if code != 0 || !strings.Contains(out, want) {
		t.Fatalf("exit %d, out %q", code, out)
	}
	code, out, _ = runAvailable(t, "--harness", "claude-code", "--json")
	if code != 0 || !strings.Contains(out, `"fallback": true`) || !strings.Contains(out, "catalog-fallback") {
		t.Fatalf("claude-code stand-in: exit %d\n%s", code, out)
	}
}

func TestModelsAvailableUnknownHarnessListsValidIDs(t *testing.T) {
	availableState(t)
	code, out, errs := runAvailable(t, "--harness", "nope")
	if code != 2 || out != "" {
		t.Fatalf("exit %d, out %q", code, out)
	}
	for _, id := range []string{"claude-code", "cursor", "codex", "kirocrew", "antigravity", "opencode"} {
		if !strings.Contains(errs, id) {
			t.Errorf("error misses valid id %s: %s", id, errs)
		}
	}
	if code, _, _ := runAvailable(t); code != 2 {
		t.Errorf("missing --harness must exit 2, got %d", code)
	}
}

func TestModelsDispatchRoutesAvailable(t *testing.T) {
	availableState(t)
	if code := runModels(cmdCat, []string{"available", "--harness", "cursor"}); code != 0 {
		t.Fatalf("exit %d", code)
	}
}

func TestDoctorAvailableModels(t *testing.T) {
	f := newDoctorFixture(t)
	t.Setenv("DOWNSHIFT_DISCOVERY", "")
	t.Setenv("DOWNSHIFT_DISCOVERY_TTL", "")
	writeFile(t, filepath.Join(f.home, ".claude", "settings.json"), claudeSettings(f.bin))

	// Only the catalog stand-in: warn with the discover fix.
	c := f.check(t, buildDoctorReport(f.env), "claude-code", "available models")
	wantStatus(t, c, doctorWarn)
	if !strings.Contains(c.Fix, "downshift models discover --harness claude-code") {
		t.Errorf("fix = %q", c.Fix)
	}

	// A real source: ok.
	cache := `{"version":1,"harnesses":{"claude-code":{"fetched_at":"` + f.started.Format(time.RFC3339) +
		`","ordered":["alpha-1","beta-2"]}}}`
	writeFile(t, filepath.Join(f.home, ".downshift", "discovered.json"), cache)
	t.Setenv(paths.EnvStateDir, filepath.Join(f.home, ".downshift"))
	c = f.check(t, buildDoctorReport(f.env), "claude-code", "available models")
	wantStatus(t, c, doctorOK)
	if !strings.Contains(c.Detail, "2 from discovery") || !strings.Contains(c.Detail, "2 untiered") {
		t.Errorf("detail = %q", c.Detail)
	}
}
