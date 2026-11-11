// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package claudecode

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift-ai/internal/hookport"
)

const (
	trID  = "0123456789ab"
	trID2 = "ba9876543210"
)

func marker(id string) string { return "full output: downshift retrieve " + id }

// resultLine is a transcript line holding one tool_result whose content is the
// given value (a string, or a list of text blocks).
func resultLine(t *testing.T, typ string, content any) string {
	t.Helper()
	b, err := json.Marshal(map[string]any{
		"type": typ,
		"message": map[string]any{"role": typ, "content": []any{
			map[string]any{"type": "tool_result", "tool_use_id": "toolu_x", "content": content},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// writeResultTranscript writes lines under root/<project>/<name> with the mtime.
func writeResultTranscript(t *testing.T, root, project, name string, mtime time.Time, lines ...string) {
	t.Helper()
	dir := filepath.Join(root, project)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(p, mtime, mtime); err != nil {
		t.Fatal(err)
	}
}

func sampleFor(got []hookport.ToolResultSample, id string) []int {
	var out []int
	for _, s := range got {
		if s.ID == id {
			out = append(out, s.Bytes)
		}
	}
	return out
}

var testLimits = readLimits{maxFiles: 5, maxFileBytes: 1 << 20}

func TestReadToolResultsFindsIDAndLength(t *testing.T) {
	root := t.TempDir()
	text := "line one\n" + marker(trID)
	writeResultTranscript(t, root, "p1", "a.jsonl", time.Now(),
		resultLine(t, "user", text),
		resultLine(t, "user", []any{map[string]any{"type": "text", "text": "héllo " + marker(trID2)}}),
		resultLine(t, "assistant", marker(trID)), // not a result the model received
	)
	got, ok := readToolResults(root, []string{trID, trID2}, testLimits)
	if !ok {
		t.Fatal("transcripts were readable")
	}
	if n := sampleFor(got, trID); len(n) != 1 || n[0] != len(text) {
		t.Errorf("trID samples %v, want [%d]", n, len(text))
	}
	want := len("héllo " + marker(trID2)) // UTF-8 bytes, not runes
	if n := sampleFor(got, trID2); len(n) != 1 || n[0] != want {
		t.Errorf("trID2 samples %v, want [%d]", n, want)
	}
}

func TestReadToolResultsReportsEveryCandidate(t *testing.T) {
	// A pre-run marker with the same id in a longer result is a second
	// candidate; the shared check decides which one counts.
	root := t.TempDir()
	writeResultTranscript(t, root, "p1", "a.jsonl", time.Now(),
		resultLine(t, "user", marker(trID)),
		resultLine(t, "user", strings.Repeat("x", 500)+marker(trID)),
	)
	got, _ := readToolResults(root, []string{trID}, testLimits)
	if n := sampleFor(got, trID); len(n) != 2 {
		t.Fatalf("samples %v, want two candidates", n)
	}
}

func TestReadToolResultsSkipsMalformedLines(t *testing.T) {
	root := t.TempDir()
	writeResultTranscript(t, root, "p1", "a.jsonl", time.Now(),
		"not json at all "+trID,
		`{"type":"user","message":`+trID,
		`{"type":"user","message":{"content":"plain string "`+trID+`"}}`,
		resultLine(t, "user", marker(trID)),
	)
	got, ok := readToolResults(root, []string{trID}, testLimits)
	if !ok || len(sampleFor(got, trID)) != 1 {
		t.Fatalf("got %v ok=%v, want the one well-formed line", got, ok)
	}
}

func TestReadToolResultsMissingDirIsUnreadable(t *testing.T) {
	if got, ok := readToolResults(filepath.Join(t.TempDir(), "absent"), []string{trID}, testLimits); ok || got != nil {
		t.Fatalf("got %v ok=%v, want unreadable", got, ok)
	}
	if _, ok := readToolResults(t.TempDir(), []string{trID}, testLimits); ok {
		t.Fatal("a directory with no transcript is unreadable, not an empty answer")
	}
}

func TestReadToolResultsNoIDsReadsNothing(t *testing.T) {
	root := t.TempDir()
	writeResultTranscript(t, root, "p1", "a.jsonl", time.Now(), resultLine(t, "user", marker(trID)))
	if got, ok := readToolResults(root, nil, testLimits); ok || got != nil {
		t.Fatalf("got %v ok=%v", got, ok)
	}
}

func TestReadToolResultsFileCapKeepsTheTail(t *testing.T) {
	root := t.TempDir()
	old := resultLine(t, "user", marker(trID))
	pad := resultLine(t, "user", strings.Repeat("y", 2000))
	recent := resultLine(t, "user", marker(trID2))
	writeResultTranscript(t, root, "p1", "a.jsonl", time.Now(), old, pad, pad, pad, recent)
	lim := readLimits{maxFiles: 5, maxFileBytes: 3000}
	got, ok := readToolResults(root, []string{trID, trID2}, lim)
	if !ok {
		t.Fatal("readable")
	}
	if len(sampleFor(got, trID)) != 0 {
		t.Error("a line before the size cap must not be read")
	}
	if len(sampleFor(got, trID2)) != 1 {
		t.Error("the most recent lines must be read")
	}
}

func TestReadToolResultsFileCountCapMostRecentFirst(t *testing.T) {
	root := t.TempDir()
	now := time.Now()
	writeResultTranscript(t, root, "p1", "old.jsonl", now.Add(-3*time.Hour), resultLine(t, "user", marker(trID)))
	writeResultTranscript(t, root, "p2", "new.jsonl", now.Add(-1*time.Hour), resultLine(t, "user", marker(trID2)))
	got, ok := readToolResults(root, []string{trID, trID2}, readLimits{maxFiles: 1, maxFileBytes: 1 << 20})
	if !ok {
		t.Fatal("readable")
	}
	if len(sampleFor(got, trID)) != 0 || len(sampleFor(got, trID2)) != 1 {
		t.Fatalf("only the newest file may be read, got %v", got)
	}
}

func TestReadToolResultsIgnoresOtherFiles(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "p1")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte(resultLine(t, "user", marker(trID))), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, ok := readToolResults(root, []string{trID}, testLimits); ok {
		t.Fatal("only .jsonl files are transcripts")
	}
}

func TestReadToolResultsUsesTheHomeProjectsDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	writeResultTranscript(t, filepath.Join(home, ".claude", "projects"), "p1", "a.jsonl", time.Now(), resultLine(t, "user", marker(trID)))
	got, ok := ReadToolResults([]string{trID})
	if !ok || len(sampleFor(got, trID)) != 1 {
		t.Fatalf("got %v ok=%v", got, ok)
	}
}
