// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/codeintel"
)

func TestDCIEnableDryRunWritesNothing(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	var stdout, stderr bytes.Buffer
	if code := runDCI([]string{"enable", "observe", "--budget=4000", "--dry-run"}, nil, &stdout, &stderr); code != 0 {
		t.Fatalf("enable --dry-run = %d, stderr: %s", code, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(dir, ".downshift")); !os.IsNotExist(err) {
		t.Fatalf("dry run created .downshift: %v", err)
	}
	out := stdout.String()
	for _, want := range []string{
		"nothing written",
		"mode off -> observe, budget 4000 bytes",
		`"mode": "observe"`,
		"claude-code: UserPromptSubmit -> downshift dci hook claude-code",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("dry-run output missing %q:\n%s", want, out)
		}
	}
}

func TestDCIEnableDryRunKeepsExistingConfig(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	cfg := codeintel.DefaultConfig()
	cfg.Mode = codeintel.ModeActive
	if err := codeintel.SaveConfig(dir, cfg); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(codeintel.ConfigPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := runDCI([]string{"enable", "observe", "--dry-run"}, nil, &stdout, &stderr); code != 0 {
		t.Fatalf("enable --dry-run = %d, stderr: %s", code, stderr.String())
	}
	after, err := os.ReadFile(codeintel.ConfigPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatalf("dry run changed the config:\n%s", after)
	}
	if !strings.Contains(stdout.String(), "mode active -> observe") {
		t.Errorf("dry run must report the current mode:\n%s", stdout.String())
	}
}
