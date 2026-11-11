// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package claudecode

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift-ai/internal/hookport"
)

const (
	trID  = "0123456789ab"
	trID2 = "ba9876543210"
)

var trBase = time.Date(2026, 11, 11, 10, 0, 0, 0, time.UTC)

func marker(id string) string { return "[compacted; full output: downshift retrieve " + id + "]" }

func sessionMarker(id string) string {
	return "downshift retrieve --session=aaaaaaaaaaaa " + id
}

// resultLine is a transcript line holding one tool_result whose content is the
// given value (a string, or a list of text blocks).
func resultLine(tb testing.TB, typ, useID string, ts time.Time, content any) string {
	tb.Helper()
	row := map[string]any{
		"type": typ,
		"message": map[string]any{"role": typ, "content": []any{
			resultBlock(useID, content),
		}},
	}
	if !ts.IsZero() {
		row["timestamp"] = ts.Format(time.RFC3339Nano)
	}
	b, err := json.Marshal(row)
	if err != nil {
		tb.Fatal(err)
	}
	return string(b)
}

func resultBlock(useID string, content any) map[string]any {
	return map[string]any{"type": "tool_result", "tool_use_id": useID, "content": content}
}

// writeResultTranscript writes lines under root/<project>/<name> with the mtime.
func writeResultTranscript(tb testing.TB, root, project, name string, mtime time.Time, lines ...string) {
	tb.Helper()
	dir := filepath.Join(root, project)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		tb.Fatal(err)
	}
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		tb.Fatal(err)
	}
	if err := os.Chtimes(p, mtime, mtime); err != nil {
		tb.Fatal(err)
	}
}

// countFor is the number of distinct results found for id with length n.
func countFor(res hookport.ToolResults, id string, n int) int {
	total := 0
	for _, s := range res.Samples {
		if s.ID == id && s.Bytes == n {
			total += max(s.Count, 1)
		}
	}
	return total
}

func anyFor(res hookport.ToolResults, id string) bool {
	for _, s := range res.Samples {
		if s.ID == id {
			return true
		}
	}
	return false
}

var testLimits = readLimits{maxFiles: 5, maxFileBytes: 1 << 20, maxTotalBytes: 8 << 20, timeout: time.Minute}

func TestReadToolResultsFindsIDAndLength(t *testing.T) {
	root := t.TempDir()
	plain := "line one\n" + marker(trID)
	session := "héllo, é " + sessionMarker(trID2) // multi-byte: bytes, not runes
	writeResultTranscript(t, root, "p1", "a.jsonl", trBase,
		resultLine(t, "user", "t1", trBase, plain),
		resultLine(t, "user", "t2", trBase, []any{map[string]any{"type": "text", "text": session}}),
	)
	res := readToolResults(root, []string{trID, trID2}, testLimits)
	if !res.Readable {
		t.Fatal("transcripts were readable")
	}
	if countFor(res, trID, len(plain)) != 1 {
		t.Errorf("bare hint: %+v", res.Samples)
	}
	if len(session) == len([]rune(session)) {
		t.Fatal("fixture must be multi-byte")
	}
	if countFor(res, trID2, len(session)) != 1 {
		t.Errorf("session hint with multi-byte text, want %d bytes: %+v", len(session), res.Samples)
	}
}

func TestReadToolResultsOnlyTheRetrieveHintCounts(t *testing.T) {
	root := t.TempDir()
	writeResultTranscript(t, root, "p1", "a.jsonl", trBase,
		resultLine(t, "user", "t1", trBase, "the id "+trID+" alone"),
		resultLine(t, "user", "t2", trBase, "downshift retrieve "+trID+"f"), // 13 hex: another id
		resultLine(t, "user", "t3", trBase, "downshift retrieve "+trID2),    // not asked for
		resultLine(t, "assistant", "t4", trBase, marker(trID)),              // not a result the model received
	)
	res := readToolResults(root, []string{trID}, testLimits)
	if !res.Readable || len(res.Samples) != 0 {
		t.Fatalf("got %+v, want readable with no sample", res)
	}
}

func TestReadToolResultsCountsDistinctResultsOnce(t *testing.T) {
	root := t.TempDir()
	text := marker(trID)
	l1 := resultLine(t, "user", "t1", trBase, text)
	l2 := resultLine(t, "user", "t2", trBase, text)
	// A resumed session copies the history of another file: same tool_use_id.
	writeResultTranscript(t, root, "p1", "a.jsonl", trBase, l1, l2)
	writeResultTranscript(t, root, "p2", "b.jsonl", trBase.Add(time.Minute), l1)
	res := readToolResults(root, []string{trID}, testLimits)
	if n := countFor(res, trID, len(text)); n != 2 {
		t.Fatalf("count %d, want 2 distinct results", n)
	}
	if len(res.Samples) != 1 {
		t.Fatalf("one (id, length) pair must be one sample: %+v", res.Samples)
	}
}

func TestReadToolResultsReportsEveryLength(t *testing.T) {
	// The pre-run marker with the same id in a longer result is a second
	// candidate; the shared check decides which one counts.
	root := t.TempDir()
	long := strings.Repeat("x", 500) + marker(trID)
	writeResultTranscript(t, root, "p1", "a.jsonl", trBase,
		resultLine(t, "user", "t1", trBase, marker(trID)),
		resultLine(t, "user", "t2", trBase, long),
	)
	res := readToolResults(root, []string{trID}, testLimits)
	if countFor(res, trID, len(marker(trID))) != 1 || countFor(res, trID, len(long)) != 1 {
		t.Fatalf("got %+v", res.Samples)
	}
}

func TestReadToolResultsSkipsMalformedLines(t *testing.T) {
	root := t.TempDir()
	writeResultTranscript(t, root, "p1", "a.jsonl", trBase,
		"not json at all downshift retrieve "+trID,
		`{"type":"user","message":downshift retrieve `+trID,
		`{"type":"user","message":{"content":"plain string downshift retrieve `+trID+`"}}`,
		resultLine(t, "user", "t1", trBase, marker(trID)),
	)
	res := readToolResults(root, []string{trID}, testLimits)
	if !res.Readable || countFor(res, trID, len(marker(trID))) != 1 {
		t.Fatalf("got %+v, want the one well-formed line", res)
	}
}

func TestReadToolResultsMissingDirIsUnreadable(t *testing.T) {
	if res := readToolResults(filepath.Join(t.TempDir(), "absent"), []string{trID}, testLimits); res.Readable || res.Samples != nil {
		t.Fatalf("got %+v, want unreadable", res)
	}
	if res := readToolResults(t.TempDir(), []string{trID}, testLimits); res.Readable {
		t.Fatal("a directory with no transcript is unreadable, not an empty answer")
	}
}

func TestReadToolResultsNoIDsReadsNothing(t *testing.T) {
	root := t.TempDir()
	writeResultTranscript(t, root, "p1", "a.jsonl", trBase, resultLine(t, "user", "t1", trBase, marker(trID)))
	if res := readToolResults(root, nil, testLimits); res.Readable || res.Samples != nil {
		t.Fatalf("got %+v", res)
	}
}

func TestReadToolResultsIgnoresOtherFiles(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "p1")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	line := resultLine(t, "user", "t1", trBase, marker(trID))
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte(line), 0o644); err != nil {
		t.Fatal(err)
	}
	if res := readToolResults(root, []string{trID}, testLimits); res.Readable {
		t.Fatal("only .jsonl files are transcripts")
	}
}

func TestReadToolResultsCoverageOfACompleteRead(t *testing.T) {
	root := t.TempDir()
	writeResultTranscript(t, root, "p1", "a.jsonl", trBase.Add(time.Hour),
		resultLine(t, "user", "t1", trBase.Add(10*time.Minute), "first"),
		resultLine(t, "user", "t2", trBase.Add(50*time.Minute), marker(trID)))
	writeResultTranscript(t, root, "p2", "b.jsonl", trBase.Add(2*time.Hour),
		resultLine(t, "user", "t3", trBase.Add(70*time.Minute), "first"))
	res := readToolResults(root, []string{trID}, testLimits)
	if res.Truncated {
		t.Error("nothing was cut")
	}
	if !res.Since.Equal(trBase.Add(10 * time.Minute)) {
		t.Errorf("since %v, want the oldest line read", res.Since)
	}
}

func TestReadToolResultsFileCapKeepsTheTail(t *testing.T) {
	root := t.TempDir()
	old := resultLine(t, "user", "t0", trBase, marker(trID))
	pad := func(i int) string {
		return resultLine(t, "user", fmt.Sprintf("p%d", i), trBase.Add(time.Duration(i)*time.Minute), strings.Repeat("y", 2000))
	}
	recent := resultLine(t, "user", "t9", trBase.Add(time.Hour), marker(trID2))
	writeResultTranscript(t, root, "p1", "a.jsonl", trBase.Add(2*time.Hour), old, pad(1), pad(2), pad(3), recent)
	lim := testLimits
	lim.maxFileBytes = 3500
	res := readToolResults(root, []string{trID, trID2}, lim)
	if !res.Readable || !res.Truncated {
		t.Fatalf("got %+v, want a readable, truncated read", res)
	}
	if anyFor(res, trID) {
		t.Error("a line before the size cap must not be read")
	}
	if !anyFor(res, trID2) {
		t.Error("the most recent lines must be read")
	}
	if !res.Since.After(trBase) {
		t.Errorf("since %v must be after the dropped head", res.Since)
	}
}

func TestReadToolResultsFileCountCapMostRecentFirst(t *testing.T) {
	root := t.TempDir()
	old := trBase.Add(-3 * time.Hour)
	writeResultTranscript(t, root, "p1", "old.jsonl", old, resultLine(t, "user", "t1", old, marker(trID)))
	newest := trBase.Add(-time.Hour)
	writeResultTranscript(t, root, "p2", "new.jsonl", newest, resultLine(t, "user", "t2", newest, marker(trID2)))
	lim := testLimits
	lim.maxFiles = 1
	res := readToolResults(root, []string{trID, trID2}, lim)
	if !res.Readable || !res.Truncated {
		t.Fatalf("got %+v", res)
	}
	if anyFor(res, trID) || !anyFor(res, trID2) {
		t.Fatalf("only the newest file may be read, got %+v", res.Samples)
	}
	if res.Since.Before(old) || !res.Since.Equal(newest) {
		t.Errorf("since %v, want the oldest line read (%v), never before the skipped file's mtime %v", res.Since, newest, old)
	}
}

func TestReadToolResultsTotalByteCap(t *testing.T) {
	root := t.TempDir()
	pad := strings.Repeat("z", 1500)
	for i, name := range []string{"a.jsonl", "b.jsonl", "c.jsonl"} {
		at := trBase.Add(time.Duration(i) * time.Hour)
		writeResultTranscript(t, root, "p", name, at, resultLine(t, "user", name, at, pad+marker(trID)))
	}
	lim := testLimits
	lim.maxTotalBytes = 2500 // room for one file and a bit
	res := readToolResults(root, []string{trID}, lim)
	if !res.Truncated || !res.Readable {
		t.Fatalf("got %+v", res)
	}
	if n := countFor(res, trID, len(pad)+len(marker(trID))); n != 1 {
		t.Fatalf("only the newest file fits the budget, got %d results", n)
	}
}

func TestReadToolResultsDeadline(t *testing.T) {
	root := t.TempDir()
	for i, name := range []string{"a.jsonl", "b.jsonl", "c.jsonl"} {
		at := trBase.Add(time.Duration(i) * time.Hour)
		writeResultTranscript(t, root, "p", name, at, resultLine(t, "user", name, at, marker(trID)))
	}
	clock := trBase
	lim := testLimits
	lim.timeout = 3 * time.Second
	lim.now = func() time.Time { clock = clock.Add(2 * time.Second); return clock }
	res := readToolResults(root, []string{trID}, lim)
	if !res.Truncated || !res.Readable {
		t.Fatalf("got %+v, want the deadline to cut the read after the first file", res)
	}
	if n := countFor(res, trID, len(marker(trID))); n != 1 {
		t.Fatalf("only the newest file was reached, got %d", n)
	}
	skippedMtime := trBase.Add(time.Hour)
	if res.Since.Before(skippedMtime) || !res.Since.Equal(trBase.Add(2*time.Hour)) {
		t.Errorf("since %v, want the oldest line read, never before the newest unread file's mtime %v", res.Since, skippedMtime)
	}
}

func TestReadToolResultsRepeatedIDsStayBounded(t *testing.T) {
	// One line with thousands of blocks that all repeat every id.
	const ids, blocks = 50, 3000
	var want []string
	var hint []string
	for i := 0; i < ids; i++ {
		id := fmt.Sprintf("%012x", i+1)
		want = append(want, id)
		hint = append(hint, "downshift retrieve "+id)
	}
	text := strings.Join(hint, "\n")
	content := make([]any, 0, blocks)
	for i := 0; i < blocks; i++ {
		content = append(content, resultBlock(fmt.Sprintf("toolu_%d", i), text))
	}
	b, err := json.Marshal(map[string]any{"type": "user", "message": map[string]any{"content": content}})
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	writeResultTranscript(t, root, "p", "a.jsonl", trBase, string(b))
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	lim := testLimits
	lim.maxFileBytes, lim.maxTotalBytes = 64<<20, 64<<20 // the line alone is several MiB
	res := readToolResults(root, want, lim)
	runtime.ReadMemStats(&after)
	if len(res.Samples) != ids {
		t.Fatalf("%d samples for %d ids: one (id, length) pair must be one sample", len(res.Samples), ids)
	}
	grown := after.TotalAlloc - before.TotalAlloc
	t.Logf("allocated %d MiB for a %d MiB line", grown>>20, len(b)>>20)
	if grown > 256<<20 {
		t.Fatalf("allocated %d MiB", grown>>20)
	}
}

func TestReadToolResultsUsesTheHomeProjectsDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	writeResultTranscript(t, filepath.Join(home, ".claude", "projects"), "p1", "a.jsonl", trBase,
		resultLine(t, "user", "t1", trBase, marker(trID)))
	res := ReadToolResults([]string{trID})
	if !res.Readable || countFor(res, trID, len(marker(trID))) != 1 {
		t.Fatalf("got %+v", res)
	}
}
