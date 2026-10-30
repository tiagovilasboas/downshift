// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestMetrics_AppendSummarize(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", EventsFileName)
	now := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)
	for _, r := range []Result{
		{Family: FamilyGoTest, RawBytes: 4000, EmittedBytes: 40, ExitClass: ExitClassZero, Compacted: true},
		{Family: FamilyGitStatus, RawBytes: 100, EmittedBytes: 100, ExitClass: ExitClassNonZero},
		{Family: FamilyNone, ExitClass: ExitClassNotFound},
	} {
		if err := Append(path, EventFrom(r, now)); err != nil {
			t.Fatal(err)
		}
	}
	s, err := Summarize(path)
	if err != nil {
		t.Fatal(err)
	}
	if s.Commands != 3 || s.Compacted != 1 || s.Failures != 2 || s.SavedBytes != 3960 || s.EstimatedTokensSaved != 990 {
		t.Fatalf("summary = %+v", s)
	}
	if s.ByFamily[FamilyGoTest] != 1 || s.ByFamily["other"] != 1 {
		t.Fatalf("by family = %v", s.ByFamily)
	}
	if s.SavedBytesByFamily[FamilyGoTest] != 3960 {
		t.Fatalf("saved bytes by family = %v", s.SavedBytesByFamily)
	}
	if s.CompactedByFamily[FamilyGoTest] != 1 {
		t.Fatalf("compacted by family = %v", s.CompactedByFamily)
	}
	if s.FailuresByFamily[FamilyGitStatus] != 1 || s.FailuresByFamily["other"] != 1 {
		t.Fatalf("failures by family = %v", s.FailuresByFamily)
	}
	if !strings.HasPrefix(s.TokenEstimate, "estimated") {
		t.Fatalf("token figure must be labelled estimated: %q", s.TokenEstimate)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	// POSIX file permissions do not apply on Windows (Go reports 0666/0444),
	// so only assert the 0600 mode on Unix-like systems.
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %v, want 0600", info.Mode().Perm())
	}
}

// The record has exactly the prompt-free fields: no command, args, path or
// output can be stored because the type has nowhere to put them.
func TestMetrics_RecordIsPromptFree(t *testing.T) {
	path := filepath.Join(t.TempDir(), EventsFileName)
	if err := Append(path, EventFrom(Result{Family: FamilyGoTest, ExitClass: ExitClassZero}, time.Now())); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	line := string(data)
	for _, key := range []string{`"ts"`, `"family"`, `"raw_bytes"`, `"emitted_bytes"`, `"exit_class"`, `"compacted"`} {
		if !strings.Contains(line, key) {
			t.Fatalf("missing %s in %s", key, line)
		}
	}
	if strings.Count(line, `":`) != 6 {
		t.Fatalf("unexpected extra fields: %s", line)
	}
}

func TestMetrics_MissingFileIsEmpty(t *testing.T) {
	s, err := Summarize(filepath.Join(t.TempDir(), "none.jsonl"))
	if err != nil || s.Commands != 0 {
		t.Fatalf("s=%+v err=%v", s, err)
	}
}

// The optional client field rides the event to the log when set, and stays
// absent (no "client" key) when empty, thanks to omitempty. The caller fills it
// from the harness environment; here we set it directly to check the wire form.
func TestMetrics_ClientOmitempty(t *testing.T) {
	t.Run("present", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), EventsFileName)
		ev := EventFrom(Result{Family: FamilyGoTest, ExitClass: ExitClassZero}, time.Now())
		ev.Client = "claude-desktop"
		if err := Append(path, ev); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), `"client":"claude-desktop"`) {
			t.Fatalf("line missing client key: %s", data)
		}
		evs, err := ReadEvents(path)
		if err != nil || len(evs) != 1 || evs[0].Client != "claude-desktop" {
			t.Fatalf("read back client = %+v, err = %v", evs, err)
		}
	})

	t.Run("absent when empty", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), EventsFileName)
		if err := Append(path, EventFrom(Result{Family: FamilyGoTest, ExitClass: ExitClassZero}, time.Now())); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(data), `"client"`) {
			t.Fatalf("empty client must be omitted, got: %s", data)
		}
	})
}
