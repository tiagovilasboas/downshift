// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import (
	"bytes"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// padding is just over the cap in long non-JSON lines, which readers skip.
func padding() []byte {
	line := append(bytes.Repeat([]byte("x"), 4095), '\n')
	return bytes.Repeat(line, LogRotateBytes/len(line)+1)
}

// bigLog writes one valid event then blank lines up to just past the cap.
func bigLog(t *testing.T, path string) {
	t.Helper()
	body := []byte(`{"ts":"2026-01-01T00:00:00Z","family":"git"}` + "\n")
	body = append(body, padding()...)
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestAppendRotatesAtCap(t *testing.T) {
	path := filepath.Join(t.TempDir(), EventsFileName)
	bigLog(t, path)
	if err := Append(path, Event{Family: "go"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path + ".1"); err != nil {
		t.Fatalf("no rotated generation: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Size() > 1024 {
		t.Fatalf("current log not fresh: %v %v", info, err)
	}
}

func TestAppendBelowCapKeepsFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), EventsFileName)
	if err := Append(path, Event{Family: "go"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path + ".1"); err == nil {
		t.Fatal("rotated a small log")
	}
}

func TestReadersSpanRotation(t *testing.T) {
	path := filepath.Join(t.TempDir(), EventsFileName)
	bigLog(t, path)
	if err := Append(path, Event{Family: "go"}); err != nil {
		t.Fatal(err)
	}
	evs, err := ReadEvents(path)
	if err != nil || len(evs) != 2 {
		t.Fatalf("events across rotation: %d %v", len(evs), err)
	}
	s, err := Summarize(path)
	if err != nil || s.Commands != 2 {
		t.Fatalf("summary across rotation: %+v %v", s, err)
	}
}

func TestSkipReadersSpanRotation(t *testing.T) {
	path := filepath.Join(t.TempDir(), SkipsFileName)
	row := []byte(`{"ts":"2026-01-01T00:00:00Z","reason":"metachar","metachar":"pipe"}` + "\n")
	body := append(row, padding()...)
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := AppendSkip(path, "metachar", "claude-code", time.Now()); err != nil {
		t.Fatal(err)
	}
	got, err := SummarizeSkips(path)
	if err != nil || got["metachar"] != 2 {
		t.Fatalf("skips across rotation: %v %v", got, err)
	}
	if rows, _ := ReadSkips(path); len(rows) != 2 {
		t.Fatalf("ReadSkips rows: %d", len(rows))
	}
	if cls, _ := SummarizeSkipClasses(path); cls["pipe"] != 1 {
		t.Fatalf("classes: %v", cls)
	}
}

func TestConcurrentAppendsAcrossRotationKeepLinesWhole(t *testing.T) {
	path := filepath.Join(t.TempDir(), EventsFileName)
	bigLog(t, path)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				_ = Append(path, Event{Family: "go"})
			}
		}()
	}
	wg.Wait()
	evs, err := ReadEvents(path)
	if err != nil {
		t.Fatal(err)
	}
	// 1 seeded + 160 appended; a lost rotation may drop the seed, never a
	// line cut in two (a torn line would fail to parse and be skipped).
	if len(evs) < 160 || len(evs) > 161 {
		t.Fatalf("events after concurrent rotation: %d", len(evs))
	}
}
