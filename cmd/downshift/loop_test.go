// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunLoop_Help(t *testing.T) {
	var stdout, stderr bytes.Buffer
	rc := runLoop([]string{"--help"}, &stdout, &stderr)
	if rc != 0 {
		t.Fatalf("runLoop(--help) = %d, want 0", rc)
	}
	out := stdout.String()
	if !strings.Contains(out, "downshift loop") || !strings.Contains(out, "Usage:") {
		t.Errorf("expected usage info in stdout, got %q", out)
	}
}

func TestRunLoop_Success(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("DOWNSHIFT_STATE_DIR", tempDir)

	var stdout, stderr bytes.Buffer
	rc := runLoop([]string{"--task=Unit test", "--session=sess_loop_test", "--json", "--", "true"}, &stdout, &stderr)
	if rc != 0 {
		t.Fatalf("runLoop() = %d, want 0; stderr: %s", rc, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, `"success": true`) {
		t.Errorf("stdout = %q; want success: true", out)
	}
}

func TestRunLoop_TripwireFailure(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("DOWNSHIFT_STATE_DIR", tempDir)

	var stdout, stderr bytes.Buffer
	rc := runLoop([]string{"--task=Broken task", "--iterations=1", "--sensor=false"}, &stdout, &stderr)
	if rc != 1 {
		t.Fatalf("runLoop() = %d, want 1 for failed sensor; stdout: %s", rc, stdout.String())
	}
	errStr := stderr.String()
	if !strings.Contains(errStr, "Autonomous loop stopped") {
		t.Errorf("stderr = %q; want 'Autonomous loop stopped'", errStr)
	}
}

func writeBatch(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "tasks.json")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestRunLoop_Batch(t *testing.T) {
	cases := []struct {
		name    string
		body    string
		extra   []string
		wantRC  int
		wantOut string
		wantErr string
	}{
		{name: "all pass", body: `[{"task":"a","sensor":"true"},{"task":"b","sensor":"true"}]`,
			wantRC: 0, wantOut: "2 passed, 0 failed, 0 skipped"},
		{name: "fail fast", body: `[{"task":"a","sensor":"false"},{"task":"b","sensor":"true"}]`,
			extra: []string{"--iterations=1"}, wantRC: 1, wantOut: "1 failed, 1 skipped"},
		{name: "continue on error", body: `[{"task":"a","sensor":"false"},{"task":"b","sensor":"true"}]`,
			extra: []string{"--iterations=1", "--continue-on-error"}, wantRC: 1, wantOut: "1 passed, 1 failed, 0 skipped"},
		{name: "bad file", body: `not json`, wantRC: 2, wantErr: "parse batch"},
		{name: "empty batch path", body: `[{"task":"a","sensor":"true"}]`, extra: []string{"--batch="}, wantRC: 2, wantErr: "needs a file path"},
		{name: "item without sensor", body: `[{"task":"a"}]`, wantRC: 2, wantErr: "no sensor"},
		{name: "task flag conflicts", body: `[{"task":"a"}]`, extra: []string{"--task=x"}, wantRC: 2, wantErr: "cannot be combined"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv("DOWNSHIFT_STATE_DIR", t.TempDir())
			args := append([]string{"--batch=" + writeBatch(t, c.body), "--session=sess_batch_test"}, c.extra...)
			var stdout, stderr bytes.Buffer
			if rc := runLoop(args, &stdout, &stderr); rc != c.wantRC {
				t.Fatalf("rc = %d, want %d; out: %s err: %s", rc, c.wantRC, stdout.String(), stderr.String())
			}
			if !strings.Contains(stdout.String(), c.wantOut) || !strings.Contains(stderr.String(), c.wantErr) {
				t.Errorf("stdout = %q stderr = %q", stdout.String(), stderr.String())
			}
		})
	}
}

func TestRunLoop_BatchJSONAndMissingFile(t *testing.T) {
	t.Setenv("DOWNSHIFT_STATE_DIR", t.TempDir())
	var stdout, stderr bytes.Buffer
	rc := runLoop([]string{"--batch=" + writeBatch(t, `[{"task":"a","sensor":"true"}]`), "--session=s", "--json"}, &stdout, &stderr)
	if rc != 0 || !strings.Contains(stdout.String(), `"passed": 1`) {
		t.Fatalf("rc = %d, stdout = %q", rc, stdout.String())
	}
	if rc := runLoop([]string{"--batch=/nonexistent/tasks.json"}, &stdout, &stderr); rc != 2 {
		t.Errorf("missing file rc = %d, want 2", rc)
	}
}

func TestRunLoopReport_Empty(t *testing.T) {
	t.Setenv("DOWNSHIFT_STATE_DIR", t.TempDir())
	var stdout, stderr bytes.Buffer
	if rc := runLoop([]string{"report"}, &stdout, &stderr); rc != 0 {
		t.Fatalf("rc = %d; stderr: %s", rc, stderr.String())
	}
	if !strings.Contains(stdout.String(), "no run decided yet") {
		t.Errorf("stdout = %q", stdout.String())
	}
	if rc := runLoop([]string{"report", "--bogus"}, &stdout, &stderr); rc != 2 {
		t.Errorf("unknown arg rc = %d, want 2", rc)
	}
}

func TestRunLoop_ContinueOnErrorNeedsBatch(t *testing.T) {
	t.Setenv("DOWNSHIFT_STATE_DIR", t.TempDir())
	var stdout, stderr bytes.Buffer
	if rc := runLoop([]string{"--continue-on-error", "--", "true"}, &stdout, &stderr); rc != 2 {
		t.Fatalf("rc = %d, want 2; stderr: %s", rc, stderr.String())
	}
}
