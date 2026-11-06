// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"strings"
	"testing"
)

// buildGhRunLog assembles a minimal gh run view --log transcript (JOB\tSTEP\t
// TIMESTAMP MESSAGE) from message rows for one job, padded so isGhRunLog's
// line-count and match thresholds are met.
func buildGhRunLog(job string, rows [][2]string) []byte {
	var b strings.Builder
	for i, r := range rows {
		b.WriteString(job)
		b.WriteString("\t")
		b.WriteString(r[0])
		b.WriteString("\t2026-10-31T12:00:0")
		b.WriteByte(byte('0' + i%10))
		b.WriteString(".1000000Z ")
		b.WriteString(r[1])
		b.WriteString("\n")
	}
	return []byte(b.String())
}

// jobLine returns the summary header line for a job, i.e. the line that starts
// with the ✓ / ✗ glyph and the job name.
func jobLine(t *testing.T, summary, job string) string {
	t.Helper()
	for _, l := range strings.Split(summary, "\n") {
		if strings.Contains(l, " "+job+" (") {
			return l
		}
	}
	t.Fatalf("job %q not found in summary:\n%s", job, summary)
	return ""
}

func summarize(t *testing.T, in []byte) (summary string, applied bool) {
	t.Helper()
	out, ok, reason := compressGhRunLog(in)
	t.Logf("applied=%v reason=%q", ok, reason)
	return string(out), ok
}

// --- Unhappy path 1: a real ##[error] annotation must mark the job failed ---

func TestGhRunLog_ErrorAnnotationMarksJobFailed(t *testing.T) {
	rows := [][2]string{
		{"Set up job", "Current runner version: '2.320.0'"},
		{"go test", "go test ./..."},
		{"go test", "##[error]FAIL github.com/example/pkg [build failed]"},
		{"go test", "detail line two"},
		{"go test", "detail line three"},
		{"go test", "detail line four"},
		{"go test", "detail line five"},
		{"go test", "detail line six"},
		{"go test", "detail line seven"},
		{"Complete job", "Cleaning up orphan processes"},
	}
	in := buildGhRunLog("test", rows)
	summary, applied := summarize(t, in)
	if !applied {
		t.Fatalf("expected the log to compact; it stayed raw")
	}
	line := jobLine(t, summary, "test")
	if !strings.HasPrefix(strings.TrimSpace(line), "✗") {
		t.Errorf("job with ##[error] must be marked failed (✗), got: %q", line)
	}
}

// --- Unhappy path 2: "Process completed with exit code N" (N!=0) marks failed

func TestGhRunLog_NonZeroExitCodeMarksJobFailed(t *testing.T) {
	rows := [][2]string{
		{"Set up job", "Current runner version: '2.320.0'"},
		{"build", "make build"},
		{"build", "some normal build output"},
		{"build", "more normal output"},
		{"build", "Process completed with exit code 2."},
		{"build", "trailing line one"},
		{"build", "trailing line two"},
		{"build", "trailing line three"},
		{"build", "trailing line four"},
		{"Complete job", "Cleaning up orphan processes"},
	}
	in := buildGhRunLog("test", rows)
	summary, applied := summarize(t, in)
	if !applied {
		t.Fatalf("expected the log to compact; it stayed raw")
	}
	line := jobLine(t, summary, "test")
	if !strings.HasPrefix(strings.TrimSpace(line), "✗") {
		t.Errorf("job with non-zero exit code must be marked failed (✗), got: %q", line)
	}
}

// --- Unhappy path 3: exit code 0 line must NOT mark the job failed ---------

func TestGhRunLog_ZeroExitCodeDoesNotMarkJobFailed(t *testing.T) {
	rows := [][2]string{
		{"Set up job", "Current runner version: '2.320.0'"},
		{"build", "make build"},
		{"build", "some normal build output"},
		{"build", "more normal output"},
		{"build", "Process completed with exit code 0."},
		{"build", "trailing line one"},
		{"build", "trailing line two"},
		{"build", "trailing line three"},
		{"build", "trailing line four"},
		{"Complete job", "Cleaning up orphan processes"},
	}
	in := buildGhRunLog("test", rows)
	summary, applied := summarize(t, in)
	if !applied {
		t.Fatalf("expected the log to compact; it stayed raw")
	}
	line := jobLine(t, summary, "test")
	if !strings.HasPrefix(strings.TrimSpace(line), "✓") {
		t.Errorf("job with exit code 0 must stay ✓, got: %q", line)
	}
}

// --- Happy path: a green run with a benign "warning" keeps the job green,
// and the benign line still survives in the summary (verbatim), just not as a
// failure marker.

func TestGhRunLog_BenignWarningKeepsJobGreenButPreservesLine(t *testing.T) {
	benign := "hint: to use in all of your new repositories, which will suppress this warning,"
	rows := [][2]string{
		{"Set up job", "Current runner version: '2.320.0'"},
		{"Run actions/checkout@v4", benign},
		{"go test", "go test ./..."},
		{"go test", "ok github.com/example/pkg 0.5s"},
		{"go test", "[command]/usr/bin/tar xz --warning=no-unknown-keyword --overwrite -C /tmp"},
		{"go test", "PASS compactor: a failing test keeps its exit code exit 1"},
		{"go test", "normal line a"},
		{"go test", "normal line b"},
		{"go test", "normal line c"},
		{"Complete job", "Cleaning up orphan processes"},
	}
	in := buildGhRunLog("test", rows)
	summary, applied := summarize(t, in)
	if !applied {
		t.Fatalf("expected the green log to compact; it stayed raw")
	}
	line := jobLine(t, summary, "test")
	if !strings.HasPrefix(strings.TrimSpace(line), "✓") {
		t.Errorf("green job with a benign warning must stay ✓, got: %q", line)
	}
	// Safety invariant: each benign danger line survives verbatim in the summary.
	for _, want := range []string{
		benign,
		"--warning=no-unknown-keyword",
		"PASS compactor: a failing test keeps its exit code exit 1",
	} {
		if !strings.Contains(summary, want) {
			t.Errorf("benign danger line dropped from summary: %q\nsummary:\n%s", want, summary)
		}
	}
	// It must be labelled as a NOTE, not an ISSUE (honest label).
	if strings.Contains(summary, "ISSUE:") {
		t.Errorf("benign danger lines must not be labelled ISSUE in a green run:\n%s", summary)
	}
}
