// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/adapters/claudecode"
	"github.com/tiagovilasboas/downshift/internal/core"
)

func captureStderr(fn func()) string {
	r, w, _ := os.Pipe()
	old := os.Stderr
	os.Stderr = w
	fn()
	w.Close()
	os.Stderr = old
	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}

// The hook warns on stderr, once per call, when the session is unknown.
func TestRunHookAdapter_WarnsOnceWhenSessionUnknown(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("DOWNSHIFT_EVENT_LOG", filepath.Join(t.TempDir(), "events.jsonl"))
	t.Setenv("DOWNSHIFT_SESSION_MODELS", filepath.Join(t.TempDir(), "absent.json"))
	stdin := `{"tool_name":"Task","tool_input":{"prompt":"rearchitect the payment flow across services"}}`
	var stderr string
	captureStdout(func() {
		stderr = captureStderr(func() {
			runHookAdapter(
				"claude-code",
				strings.NewReader(stdin),
				func(b []byte) (claudecode.Event, error) { var e claudecode.Event; return e, json.Unmarshal(b, &e) },
				func(e claudecode.Event) (any, string, core.Decision) { return claudecode.Handle(e, cmdCat) },
				printAllow,
			)
		})
	})
	if n := strings.Count(stderr, "no session allowlist for claude-code"); n != 1 {
		t.Fatalf("want exactly one session warning, got %d:\n%s", n, stderr)
	}
}
