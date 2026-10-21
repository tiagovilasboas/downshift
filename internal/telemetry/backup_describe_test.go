// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

func writeBackup(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestPrintBackupLine_NamesSizeAndSpan(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DOWNSHIFT_EVENT_LOG", filepath.Join(dir, "events.jsonl"))
	body := `{"timestamp":"2026-10-01T09:00:00Z","harness":"claude-code"}` + "\n" +
		`{"timestamp":"2026-10-02T10:30:00Z","harness":"claude-code"}` + "\n"
	writeBackup(t, dir, "events.jsonl.bak-test", body)

	var out bytes.Buffer
	telemetry.PrintBackupLine(&out)
	got := out.String()
	want := "Backups beside the log: events.jsonl.bak-test (" +
		strconv.Itoa(len(body)) + " bytes, 2026-10-01 to 2026-10-02).\n"
	if got != want {
		t.Fatalf("line = %q, want %q", got, want)
	}
	if strings.Contains(got, dir) {
		t.Fatalf("line leaks a full path: %q", got)
	}
}

func TestPrintBackupLine_NoBackupsNoLine(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DOWNSHIFT_EVENT_LOG", filepath.Join(dir, "events.jsonl"))
	writeBackup(t, dir, "events.jsonl", `{"timestamp":"2026-10-01T09:00:00Z"}`+"\n")

	var out bytes.Buffer
	telemetry.PrintBackupLine(&out)
	if out.Len() != 0 {
		t.Fatalf("expected no line, got %q", out.String())
	}
}

func TestPrintBackupLine_MalformedFirstLineNoSpan(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DOWNSHIFT_EVENT_LOG", filepath.Join(dir, "events.jsonl"))
	body := "not json at all\n" + `{"timestamp":"2026-10-02T10:30:00Z"}` + "\n"
	writeBackup(t, dir, "events.jsonl.bak-broken", body)

	var out bytes.Buffer
	telemetry.PrintBackupLine(&out)
	want := "Backups beside the log: events.jsonl.bak-broken (" + strconv.Itoa(len(body)) + " bytes).\n"
	if out.String() != want {
		t.Fatalf("line = %q, want %q", out.String(), want)
	}
}

func TestPrintBackupLine_CapsAtThreeNames(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DOWNSHIFT_EVENT_LOG", filepath.Join(dir, "events.jsonl"))
	for _, name := range []string{"a", "b", "c", "d"} {
		writeBackup(t, dir, "events.jsonl.bak-"+name, "")
	}

	var out bytes.Buffer
	telemetry.PrintBackupLine(&out)
	got := out.String()
	if strings.Count(got, "events.jsonl.bak-") != 3 || !strings.Contains(got, "and 1 more") {
		t.Fatalf("line = %q, want three names and 'and 1 more'", got)
	}
}
