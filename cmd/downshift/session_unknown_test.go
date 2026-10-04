// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: BUSL-1.1
// Commercial use requires a licence — see LICENSE for terms.

package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiagovilasboas/harness-downshift/internal/adapters/claudecode"
	"github.com/tiagovilasboas/harness-downshift/internal/core"
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

// Without a session allowlist the hook never rewrites, so `try` must not
// claim it would.
func TestRunTry_SessionUnknownSaysNoRewrite(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("DOWNSHIFT_SESSION_MODELS", filepath.Join(t.TempDir(), "absent.json"))
	out := captureStdout(func() {
		runTry(cmdCat, []string{"rearchitect the payment flow across services", "claude-code"})
	})
	if strings.Contains(out, "Rewrite:    yes") {
		t.Fatalf("try claimed a rewrite with no session allowlist:\n%s", out)
	}
	if !strings.Contains(out, "no: session unknown") {
		t.Fatalf("try must say the session is unknown:\n%s", out)
	}
}

// With a session allowlist `try` reports the model the hook would write.
func TestRunTry_SessionKnownReportsHookRewrite(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	frontierID := cmdCat.ModelFor("claude-code", core.TierFrontier).ID
	setSessionAllowlist(t, "claude-code", []string{
		cmdCat.ModelFor("claude-code", core.TierSmall).ID,
		cmdCat.ModelFor("claude-code", core.TierMid).ID,
		frontierID,
	})
	out := captureStdout(func() {
		runTry(cmdCat, []string{"rearchitect the payment flow across services", "claude-code"})
	})
	if !strings.Contains(out, "Rewrite:    yes → "+frontierID) {
		t.Fatalf("want rewrite to %s:\n%s", frontierID, out)
	}
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
