// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/xml"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

type plistDict struct {
	Inner []byte `xml:",innerxml"`
}

// The LaunchAgent script must write a well-formed plist even for a path with
// XML metacharacters, log under the private state dir (not /tmp), and pass the
// server no arguments it would ignore.
func TestLaunchdSetupWritesASafePlist(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("bash script for macOS")
	}
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not available")
	}
	src, err := os.ReadFile("dsmon-launchd-setup.sh")
	if err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	root := filepath.Join(t.TempDir(), "a&b<c>d")
	scriptDir := filepath.Join(root, "cmd", "dsmon")
	if err := os.MkdirAll(scriptDir, 0o755); err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(scriptDir, "dsmon-launchd-setup.sh")
	if err := os.WriteFile(script, src, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "downshift"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command("bash", script)
	cmd.Env = []string{"HOME=" + home, "PATH=" + os.Getenv("PATH"), "DSMON_PLIST_ONLY=1"}
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("script failed: %v\n%s", err, out)
	}

	plistPath := filepath.Join(home, "Library", "LaunchAgents", "com.tiagovilasboas.dsmon-server.plist")
	raw, err := os.ReadFile(plistPath)
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Dict plistDict `xml:"dict"`
	}
	if err := xml.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("plist is not well-formed XML: %v\n%s", err, raw)
	}
	text := string(raw)
	if !strings.Contains(text, "a&amp;b&lt;c&gt;d") {
		t.Errorf("the binary path is not XML-escaped:\n%s", text)
	}
	for _, bad := range []string{"--events", "--agents", "--web", "/tmp/dsmon-server.log", ".harness-downshift"} {
		if strings.Contains(text, bad) {
			t.Errorf("plist still contains %q:\n%s", bad, text)
		}
	}
	logDir := filepath.Join(home, ".downshift", "logs")
	if !strings.Contains(text, filepath.Join(logDir, "dsmon-server.log")) {
		t.Errorf("log is not under %s:\n%s", logDir, text)
	}
	for path, want := range map[string]os.FileMode{
		filepath.Join(home, ".downshift"): 0o700, logDir: 0o700, plistPath: 0o600,
	} {
		if fi, err := os.Stat(path); err != nil || fi.Mode().Perm() != want {
			t.Errorf("%s = %v %v, want %v", path, fi, err, want)
		}
	}
}
