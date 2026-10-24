// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package execwrap

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// Every exec run that is not compacted names why; a compacted run names none.
func TestRun_PassthroughReason(t *testing.T) {
	cases := []struct {
		name        string
		bin         string
		script      string
		argv        []string
		wantCompact bool
		wantReason  string
	}{
		{
			name: "compacted run has no reason", bin: "go",
			script: "printf '" + strings.ReplaceAll(passingSuite, "\t", "\\t") + "'",
			argv:   []string{"go", "test", "./..."}, wantCompact: true, wantReason: "",
		},
		{
			name: "empty output is small", bin: "go",
			script: "true", argv: []string{"go", "test"}, wantReason: PassthroughSmallOutput,
		},
		{
			name: "summary not shorter than raw is small", bin: "go",
			script: "printf 'ok  \\tgithub.com/x/internal/a\\t0.1s\\nok  \\tgithub.com/x/internal/b\\t0.1s\\n'",
			argv:   []string{"go", "test"}, wantReason: PassthroughSmallOutput,
		},
		{
			name: "failing run is nonzero_exit", bin: "go",
			script: "printf '" + strings.ReplaceAll(passingSuite, "\t", "\\t") + "'; exit 1",
			argv:   []string{"go", "test"}, wantReason: PassthroughNonZeroExit,
		},
		{
			name: "unrecognised shape is unrecognized_output", bin: "go",
			script: "echo 'hello world'", argv: []string{"go", "test"}, wantReason: PassthroughUnrecognized,
		},
		{
			name: "family without a compressor is unrecognized_output", bin: "git",
			script: "printf 'commit 0123456789abcdef0123456789abcdef01234567\\nAuthor: A <a@example.com>\\n'",
			argv:   []string{"git", "log"}, wantReason: PassthroughUnrecognized,
		},
		{
			name: "not allowlisted is refused", argv: []string{"rm", "-rf", "x"}, wantReason: PassthroughRefused,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.bin != "" {
				fakeBin(t, tc.bin, tc.script)
			}
			r, _, _ := run(t, tc.argv, true)
			if r.Compacted != tc.wantCompact {
				t.Fatalf("compacted = %v, want %v (%+v)", r.Compacted, tc.wantCompact, r)
			}
			if r.Passthrough != tc.wantReason {
				t.Fatalf("passthrough = %q, want %q (%+v)", r.Passthrough, tc.wantReason, r)
			}

			ev := EventFrom(r, time.Now())
			if ev.Passthrough != tc.wantReason {
				t.Fatalf("event passthrough = %q, want %q", ev.Passthrough, tc.wantReason)
			}
			line, err := json.Marshal(ev)
			if err != nil {
				t.Fatal(err)
			}
			hasField := strings.Contains(string(line), `"passthrough"`)
			if hasField != (tc.wantReason != "") {
				t.Fatalf("passthrough key in %s: got %v, want %v", line, hasField, tc.wantReason != "")
			}
		})
	}
}
