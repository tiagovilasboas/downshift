// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/paths"
	"github.com/tiagovilasboas/downshift/internal/testenv"
)

// cleanConfigEnv isolates every input `config` reads.
func cleanConfigEnv(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	testenv.SetHome(t, home)
	for _, k := range []string{paths.EnvStateDir, "DOWNSHIFT_QUOTA_MODE",
		"DOWNSHIFT_QUIET", "DOWNSHIFT_SEMANTIC_BOOST", "DOWNSHIFT_MINILM",
		"DOWNSHIFT_SESSION_MODELS", "DOWNSHIFT_DISCOVERED", "DOWNSHIFT_CURSOR_NATIVE_FILE", "DOWNSHIFT_DISCOVERY"} {
		t.Setenv(k, "")
	}
	return home
}

func configEntries(t *testing.T) map[string]configEntry {
	t.Helper()
	var b bytes.Buffer
	if rc := runConfig([]string{"--json"}, &b, &bytes.Buffer{}, cmdCat); rc != 0 {
		t.Fatalf("rc = %d", rc)
	}
	var rep configReport
	if err := json.Unmarshal(b.Bytes(), &rep); err != nil {
		t.Fatal(err)
	}
	m := map[string]configEntry{}
	for _, e := range rep.Settings {
		k := e.Key
		if e.Harness != "" {
			k += "." + e.Harness
		}
		m[k] = e
	}
	return m
}

func TestConfig_DefaultsNameTheirSource(t *testing.T) {
	cleanConfigEnv(t)
	m := configEntries(t)
	want := map[string][2]string{
		"quota_mode":     {"default", cfgDefault},
		"quiet":          {"off", cfgDefault},
		"catalog":        {"embedded", cfgDefault},
		"semantic_boost": {"on (hash embedder)", cfgDefault},
	}
	for k, w := range want {
		if e, ok := m[k]; !ok || e.Value != w[0] || e.Source != w[1] {
			t.Errorf("%s = %+v, want value %q source %q", k, e, w[0], w[1])
		}
	}
	if e := m["state_dir"]; e.Source != cfgDefault || !strings.HasSuffix(e.Value, ".downshift") {
		t.Errorf("state_dir = %+v", e)
	}
	if e := m["session_models.claude-code"]; e.Source != cfgDefault || !strings.Contains(e.Value, "rewrites held") {
		t.Errorf("claude-code session = %+v", e)
	}
	if e := m["session_models.codex"]; e.Source != cfgDefault || !strings.Contains(e.Value, "none") {
		t.Errorf("codex session = %+v", e)
	}
}

func TestConfig_EnvAndFileSources(t *testing.T) {
	cleanConfigEnv(t)
	state := t.TempDir()
	t.Setenv(paths.EnvStateDir, state)
	t.Setenv("DOWNSHIFT_QUOTA_MODE", "required")
	t.Setenv("DOWNSHIFT_QUIET", "1")
	t.Setenv("DOWNSHIFT_MINILM", "off")
	writeFile(t, filepath.Join(state, "catalog.json"), "{}")
	writeFile(t, filepath.Join(state, "session-models.json"), `{"codex":["gpt-x","gpt-y"]}`)

	m := configEntries(t)
	want := map[string][2]string{
		"state_dir":      {state, cfgEnv},
		"quota_mode":     {"required", cfgEnv},
		"quiet":          {"on", cfgEnv},
		"semantic_boost": {"off", cfgEnv},
	}
	for k, w := range want {
		if e := m[k]; e.Value != w[0] || e.Source != w[1] {
			t.Errorf("%s = %+v, want value %q source %q", k, e, w[0], w[1])
		}
	}
	if e := m["catalog"]; e.Source != cfgFile || !strings.Contains(e.Value, "catalog.json") {
		t.Errorf("catalog = %+v", e)
	}
	if e := m["session_models.codex"]; e.Source != cfgFile || !strings.Contains(e.Value, "2 models") {
		t.Errorf("codex session = %+v", e)
	}
	// The env var that points at the file wins the label.
	t.Setenv("DOWNSHIFT_SESSION_MODELS", filepath.Join(state, "session-models.json"))
	if e := configEntries(t)["session_models.codex"]; e.Source != cfgEnv {
		t.Errorf("DOWNSHIFT_SESSION_MODELS must report env, got %+v", e)
	}
	// The retired control-group variable is not an entry any more.
	t.Setenv("DOWNSHIFT_CONTROL_SHARE", "0.25")
	if _, ok := configEntries(t)["control_share"]; ok {
		t.Error("config must not list control_share")
	}
}

func TestConfig_TextAndUsage(t *testing.T) {
	cleanConfigEnv(t)
	var out, errOut bytes.Buffer
	if rc := runConfig(nil, &out, &errOut, cmdCat); rc != 0 {
		t.Fatalf("rc = %d", rc)
	}
	for _, s := range []string{"state_dir", "quota_mode", "[default]", "session_models.claude-code"} {
		if !strings.Contains(out.String(), s) {
			t.Errorf("text output lacks %q:\n%s", s, out.String())
		}
	}
	if rc := runConfig([]string{"extra"}, &out, &errOut, cmdCat); rc != 2 {
		t.Errorf("positional arg: rc = %d", rc)
	}
}
