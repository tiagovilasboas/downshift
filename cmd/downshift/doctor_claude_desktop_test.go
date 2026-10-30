// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// claudeConfigDir returns the Claude app config directory under a fake HOME,
// mirroring os.UserConfigDir for the current OS. It returns "" where the test
// cannot control the resolved config dir, so the test skips there.
func claudeConfigDir(t *testing.T, home string) string {
	t.Helper()
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", "Claude")
	case "linux":
		return filepath.Join(home, ".config", "Claude")
	default:
		return ""
	}
}

func TestCheckClaudeDesktopSurfaces(t *testing.T) {
	// Isolate HOME and the state dir, as the repo's doctor tests do, so
	// os.UserConfigDir resolves inside the temp tree.
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("DOWNSHIFT_STATE_DIR", filepath.Join(home, "state"))
	if runtime.GOOS == "linux" {
		t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	}

	cfgDir := claudeConfigDir(t, home)
	if cfgDir == "" {
		t.Skipf("no known Claude config dir for GOOS %q", runtime.GOOS)
	}

	collect := func() map[string]string {
		got := map[string]string{}
		checkClaudeDesktopSurfaces(func(id, _, detail, _ string) {
			got[id] = detail
		}, "claude-code")
		return got
	}

	// Without the Claude directory, no surface is reported.
	if got := collect(); len(got) != 0 {
		t.Errorf("without Claude installed, reported %v; want nothing", got)
	}

	// A non-claude-code harness never reports these surfaces, even installed.
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatalf("make Claude dir: %v", err)
	}
	for _, h := range []string{"kirocrew", "cursor", "codex"} {
		checkClaudeDesktopSurfaces(func(id, _, _, _ string) {
			t.Errorf("%s reported surface %s", h, id)
		}, h)
	}

	// With the Claude directory present on the claude-code check, both
	// surfaces are reported with their feature statuses.
	got := collect()
	for _, id := range []string{"claude-desktop-chat", "claude-desktop-cowork"} {
		detail, ok := got[id]
		if !ok {
			t.Errorf("Claude installed but surface %s not reported", id)
			continue
		}
		if !strings.Contains(detail, "model_rewrite:none") {
			t.Errorf("%s detail = %q; want it to mention model_rewrite:none", id, detail)
		}
	}
	if !strings.Contains(got["claude-desktop-chat"], "mcp:partial") {
		t.Errorf("chat detail = %q; want mcp:partial", got["claude-desktop-chat"])
	}
	if !strings.Contains(got["claude-desktop-cowork"], "mcp:unverified") {
		t.Errorf("cowork detail = %q; want mcp:unverified", got["claude-desktop-cowork"])
	}
}
