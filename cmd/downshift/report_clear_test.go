// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var clearNow = time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)

// clearState makes an isolated state dir with two measurement logs and two
// files that must survive a clear.
func clearState(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("DOWNSHIFT_STATE_DIR", dir)
	for name, body := range map[string]string{
		"events.jsonl":        `{"outcome":"allow"}` + "\n",
		"exec-events.jsonl":   `{"family":"go-test"}` + "\n",
		"config.yaml":         "mode: balanced\n",
		"session-models.json": "{}\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func clearExists(path string) bool { _, err := os.Stat(path); return err == nil }

func TestReportClearMovesMeasurementLogsAndKeepsConfig(t *testing.T) {
	dir := clearState(t)
	var out, errOut bytes.Buffer
	if code := runReportClear([]string{"--yes"}, strings.NewReader(""), &out, &errOut, clearNow); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	backup := filepath.Join(dir, "backups", "report-clear-20261031T120000Z")
	for _, name := range []string{"events.jsonl", "exec-events.jsonl"} {
		if clearExists(filepath.Join(dir, name)) {
			t.Errorf("%s is still in the state dir", name)
		}
		if !clearExists(filepath.Join(backup, name)) {
			t.Errorf("%s was not moved to the backup", name)
		}
	}
	for _, name := range []string{"config.yaml", "session-models.json"} {
		if !clearExists(filepath.Join(dir, name)) {
			t.Errorf("%s must be kept", name)
		}
	}
	if !strings.Contains(out.String(), "Moved 2 of 2 files") || !strings.Contains(out.String(), backup) {
		t.Errorf("output should name the count and the backup: %s", out.String())
	}
}

func TestReportClearDryRunMovesNothing(t *testing.T) {
	dir := clearState(t)
	var out, errOut bytes.Buffer
	if code := runReportClear([]string{"--dry-run"}, strings.NewReader(""), &out, &errOut, clearNow); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	if !clearExists(filepath.Join(dir, "events.jsonl")) || clearExists(filepath.Join(dir, "backups")) {
		t.Error("a dry run must not move or create anything")
	}
	if !strings.Contains(out.String(), "events.jsonl") || !strings.Contains(out.String(), "Dry run") {
		t.Errorf("dry run should list the files: %s", out.String())
	}
}

// Without --yes and without a terminal (a pipe, a script), clear refuses
// rather than reading an answer nobody typed.
func TestReportClearRefusesWithoutTerminalOrYes(t *testing.T) {
	dir := clearState(t)
	var out, errOut bytes.Buffer
	if code := runReportClear(nil, strings.NewReader("y\n"), &out, &errOut, clearNow); code != 2 {
		t.Fatalf("exit %d, want 2", code)
	}
	if !clearExists(filepath.Join(dir, "events.jsonl")) {
		t.Error("nothing may move without confirmation")
	}
	if !strings.Contains(errOut.String(), "--yes") {
		t.Errorf("the refusal should name --yes: %s", errOut.String())
	}
}

func TestReportClearNothingToClear(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("DOWNSHIFT_STATE_DIR", dir)
	var out, errOut bytes.Buffer
	if code := runReportClear([]string{"--yes"}, strings.NewReader(""), &out, &errOut, clearNow); code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(out.String(), "Nothing to clear") || clearExists(filepath.Join(dir, "backups")) {
		t.Errorf("an empty state dir should say so and create nothing: %s", out.String())
	}
}

func TestReportClearRejectsUnknownFlag(t *testing.T) {
	clearState(t)
	var out, errOut bytes.Buffer
	if code := runReportClear([]string{"--force"}, strings.NewReader(""), &out, &errOut, clearNow); code != 2 {
		t.Fatalf("exit %d, want 2", code)
	}
}
