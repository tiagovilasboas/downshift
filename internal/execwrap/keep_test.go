// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package execwrap

import (
	"bytes"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/compressor"
)

func TestRun_Keep(t *testing.T) {
	passBody := compressor.CompressExit([]byte(passingSuite), compressor.ModeSafe, 0).Output
	failBody := compressor.CompressFailed([]byte(failedSuite), compressor.FormatGoTest).Output
	cases := []struct {
		name       string
		script     string
		raw        string
		keep       func(*[][]byte) func([]byte) (string, bool)
		wantCalls  int
		wantExit   int
		wantMarker string
		body       []byte
	}{
		{
			name: "compacted run keeps raw and marker carries the id", script: shellPrintf(passingSuite), raw: passingSuite,
			keep: func(c *[][]byte) func([]byte) (string, bool) {
				return func(b []byte) (string, bool) { *c = append(*c, b); return "abc123def456", true }
			},
			wantCalls: 1, wantMarker: MarkerWithID(len(passingSuite), len(passBody), "abc123def456"), body: passBody,
		},
		{
			name: "failed run compacted by the failure compressor counts", script: shellPrintf(failedSuite) + "; exit 1", raw: failedSuite,
			keep: func(c *[][]byte) func([]byte) (string, bool) {
				return func(b []byte) (string, bool) { *c = append(*c, b); return "000000000001", true }
			},
			wantCalls: 1, wantExit: 1, wantMarker: MarkerWithID(len(failedSuite), len(failBody), "000000000001"), body: failBody,
		},
		{
			name: "keep false keeps the old marker", script: shellPrintf(passingSuite), raw: passingSuite,
			keep: func(c *[][]byte) func([]byte) (string, bool) {
				return func(b []byte) (string, bool) { *c = append(*c, b); return "x", false }
			},
			wantCalls: 1, wantMarker: Marker(len(passingSuite), len(passBody)), body: passBody,
		},
		{
			name: "keep panics keeps the old marker and the exit code", script: shellPrintf(failedSuite) + "; exit 1", raw: failedSuite,
			keep: func(c *[][]byte) func([]byte) (string, bool) {
				return func(b []byte) (string, bool) { *c = append(*c, b); panic("boom") }
			},
			wantCalls: 1, wantExit: 1, wantMarker: Marker(len(failedSuite), len(failBody)), body: failBody,
		},
		{
			name: "nil keep matches today", script: shellPrintf(passingSuite), raw: passingSuite,
			keep:       func(*[][]byte) func([]byte) (string, bool) { return nil },
			wantMarker: Marker(len(passingSuite), len(passBody)), body: passBody,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fakeBin(t, "go", tc.script)
			var calls [][]byte
			var out, errb bytes.Buffer
			r := Run([]string{"go", "test", "./..."}, Options{Stdin: strings.NewReader(""), Stdout: &out, Stderr: &errb, Compact: true, Keep: tc.keep(&calls)})
			if r.ExitCode != tc.wantExit || !r.Compacted {
				t.Fatalf("exit=%d compacted=%v", r.ExitCode, r.Compacted)
			}
			if len(calls) != tc.wantCalls || (len(calls) == 1 && string(calls[0]) != tc.raw) {
				t.Fatalf("keep calls=%d, want %d with the exact raw bytes", len(calls), tc.wantCalls)
			}
			if want := string(tc.body) + tc.wantMarker; out.String() != want {
				t.Errorf("out = %q, want %q", out.String(), want)
			}
		})
	}
}

func TestRun_KeepNotCalledOnPassthrough(t *testing.T) {
	cases := []struct {
		name, script string
		compact      bool
		wantExit     int
	}{
		{"small output", "printf 'ok  \\tx\\t0.1s\\n'", true, 0},
		{"compaction off", shellPrintf(passingSuite), false, 0},
		{"non-zero exit stays raw on stderr", shellPrintf(failedSuite) + "; echo oops >&2; exit 1", true, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fakeBin(t, "go", tc.script)
			called := false
			var out, errb bytes.Buffer
			r := Run([]string{"go", "test"}, Options{Stdin: strings.NewReader(""), Stdout: &out, Stderr: &errb, Compact: tc.compact,
				Keep: func([]byte) (string, bool) { called = true; return "id", true }})
			if called || r.Compacted || r.ExitCode != tc.wantExit {
				t.Fatalf("called=%v compacted=%v exit=%d", called, r.Compacted, r.ExitCode)
			}
		})
	}
}
