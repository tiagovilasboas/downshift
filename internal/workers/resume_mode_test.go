// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package workers

import (
	"strings"
	"testing"
)

// TestResumeKeepsTheLaunchModeModelAndEffort drives read and write for both CLIs
// and checks the resume argv carries the same mode, sandbox, model and effort as
// the first launch, plus the MCP-off override.
func TestResumeKeepsTheLaunchModeModelAndEffort(t *testing.T) {
	spec := Spec{Dir: t.TempDir(), Model: "m-1", Effort: "high"}
	cases := []struct {
		name     string
		plan     planner
		launch   []string // must appear on the launch argv, as a joined pair
		resume   []string // must appear on the resume argv, as a joined pair
		notOnRun []string
	}{
		{"claude read", claudeReadOnly, []string{"--permission-mode plan", "--model m-1", "--effort high"}, []string{"--permission-mode plan", "--model m-1", "--effort high"}, []string{"bypassPermissions"}},
		{"claude write", claudeWrite, []string{"--permission-mode bypassPermissions", "--model m-1", "--effort high"}, []string{"--permission-mode bypassPermissions", "--model m-1", "--effort high"}, nil},
		{"codex read", codexReadOnly, []string{"--sandbox read-only", "--model m-1", "model_reasoning_effort=high", "mcp_servers={}"}, []string{`sandbox_mode="read-only"`, "--model m-1", "model_reasoning_effort=high", "mcp_servers={}"}, []string{"workspace-write", "approval_policy"}},
		{"codex write", codexWrite, []string{"--sandbox workspace-write", `-c approval_policy="never"`, "--model m-1", "model_reasoning_effort=high", "mcp_servers={}"}, []string{`sandbox_mode="workspace-write"`, `approval_policy="never"`, "--model m-1", "model_reasoning_effort=high", "mcp_servers={}"}, []string{"read-only"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			a, err := c.plan(spec)
			if err != nil {
				t.Fatal(err)
			}
			if a.cleanup != nil {
				defer a.cleanup()
			}
			launch := strings.Join(a.args, " ")
			for _, want := range c.launch {
				if !strings.Contains(launch, want) {
					t.Errorf("launch argv misses %q: %s", want, launch)
				}
			}
			resume := strings.Join(a.resume("sid-1"), " ")
			for _, want := range c.resume {
				if !strings.Contains(resume, want) {
					t.Errorf("resume argv misses %q: %s", want, resume)
				}
			}
			for _, bad := range c.notOnRun {
				if strings.Contains(resume, bad) {
					t.Errorf("resume argv has %q: %s", bad, resume)
				}
			}
		})
	}
}

// TestWorkerProcessCarriesTheMarkerEnv runs a shim and checks the child sees
// DOWNSHIFT_WORKER=1 even when the spec has no env of its own.
func TestWorkerProcessCarriesTheMarkerEnv(t *testing.T) {
	out := t.TempDir() + "/env.txt"
	shim(t, "claude", `printf '%s' "$`+WorkerMarkerEnv+`" > '`+out+`'; echo '{"type":"result","subtype":"success","result":"ok"}'`)
	p, err := cliLauncher{plan: claudeReadOnly}.Launch(Spec{Dir: t.TempDir(), Task: "go"})
	if err != nil {
		t.Fatal(err)
	}
	_ = p.Wait()
	b := readFile(t, out)
	if b != "1" {
		t.Fatalf("%s = %q, want 1", WorkerMarkerEnv, b)
	}
}
