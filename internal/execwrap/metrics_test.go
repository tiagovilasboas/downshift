// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import (
	"os"
	"path/filepath"
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
	if s.Commands != 3 || s.Compacted != 1 || s.SavedBytes != 3960 || s.EstimatedTokensSaved != 990 {
		t.Fatalf("summary = %+v", s)
	}
	if s.ByFamily[FamilyGoTest] != 1 || s.ByFamily["other"] != 1 {
		t.Fatalf("by family = %v", s.ByFamily)
	}
	if !strings.HasPrefix(s.TokenEstimate, "estimated") {
		t.Fatalf("token figure must be labelled estimated: %q", s.TokenEstimate)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
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
