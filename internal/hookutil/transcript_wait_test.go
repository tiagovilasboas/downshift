// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package hookutil

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func shortTranscriptWait(t *testing.T) {
	t.Helper()
	total, step := TranscriptWaitTotal, TranscriptWaitStep
	TranscriptWaitTotal, TranscriptWaitStep = 60*time.Millisecond, 10*time.Millisecond
	t.Cleanup(func() { TranscriptWaitTotal, TranscriptWaitStep = total, step })
}

func TestTranscriptWaitPoll_ReturnsOnceTheValueAppears(t *testing.T) {
	shortTranscriptWait(t)
	path := filepath.Join(t.TempDir(), "s.jsonl")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	calls := 0
	var w TranscriptWait
	got := w.Poll(path, "k", func() string {
		calls++
		if calls < 3 {
			return ""
		}
		return "model-x"
	})
	if got != "model-x" {
		t.Fatalf("got %q", got)
	}
}

func TestTranscriptWaitPoll_NoWaitWithoutFileAndOncePerKey(t *testing.T) {
	shortTranscriptWait(t)
	var w TranscriptWait
	never := func() string { t.Fatal("must not read"); return "" }
	if got := w.Poll("", "k", never); got != "" {
		t.Fatalf("empty path: %q", got)
	}
	if got := w.Poll(filepath.Join(t.TempDir(), "none.jsonl"), "k", never); got != "" {
		t.Fatalf("missing file: %q", got)
	}
	path := filepath.Join(t.TempDir(), "s.jsonl")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	empty := func() string { return "" }
	start := time.Now()
	if got := w.Poll(path, "k", empty); got != "" || time.Since(start) < 50*time.Millisecond {
		t.Fatalf("first poll must wait the bounded time, got %q after %s", got, time.Since(start))
	}
	if got := w.Poll(path, "k", never); got != "" {
		t.Fatalf("second poll of the same key: %q", got)
	}
}
