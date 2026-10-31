// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package workers

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// makeRepo creates a git repository with one commit and returns its path.
func makeRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	dir := t.TempDir()
	for _, args := range [][]string{
		{"init", "-q"},
		{"-c", "user.email=t@example.com", "-c", "user.name=t", "commit", "-q", "--allow-empty", "-m", "init"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = GitEnv(os.Environ())
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	return dir
}

// writeShim is a fake claude whose work is to record its directory and create a
// file. It writes a session id so the test can check a write worker is
// resumable too and that its session is keyed by the worktree it ran in.
const writeShim = `pwd > cwd.txt; echo changed > new.txt; echo '{"subtype":"success","is_error":false,"result":"wrote","session_id":"wt-session-1"}'`

func TestWriteJobRunsInItsOwnWorktreeAndKeepsTheCheckoutClean(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell shims")
	}
	shimDir := shim(t, "claude", writeShim)
	repo := makeRepo(t)
	q := New(Routes(ByAgent(ReadOnlyLaunchers()), ByAgent(WriteLaunchers())), 1, fixedNow)
	if _, err := q.Delegate([]Spec{{Agent: "claude", Dir: repo, Task: "change it", Budget: time.Minute, Write: true}}); err != nil {
		t.Fatal(err)
	}
	c := q.Collect(context.Background(), 0, true, 20*time.Second)
	if len(c.Deliveries) != 1 || c.Deliveries[0].Outcome != "done" {
		t.Fatalf("deliveries = %+v", c.Deliveries)
	}
	wt := filepath.Join(repo, ".downshift", "worktrees", "w001")
	if !strings.Contains(c.Deliveries[0].Text, wt) || !strings.Contains(c.Deliveries[0].Text, "downshift/w001") {
		t.Errorf("the report must name the worktree and branch: %q", c.Deliveries[0].Text)
	}
	if _, err := os.Stat(filepath.Join(wt, "new.txt")); err != nil {
		t.Errorf("the write did not land in the worktree: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo, "new.txt")); !os.IsNotExist(err) {
		t.Errorf("the write leaked into the checkout: %v", err)
	}
	// The worker ran in the worktree, not the user's checkout: its persisted
	// claude session is keyed by that directory, so it does not show in the
	// user's /resume in their own project dir. cwd.txt records where it ran.
	ran := strings.TrimSpace(readFile(t, filepath.Join(wt, "cwd.txt")))
	if evalWt, _ := filepath.EvalSymlinks(wt); ran != wt && ran != evalWt {
		t.Errorf("the write worker ran in %q, not the worktree %q", ran, wt)
	}
	// A write worker is resumable too: its session id reaches the delivery.
	if c.Deliveries[0].SessionID != "wt-session-1" {
		t.Errorf("write worker session id = %q, want wt-session-1", c.Deliveries[0].SessionID)
	}
	list := exec.Command("git", "-C", repo, "branch", "--list", "downshift/w001")
	list.Env = GitEnv(os.Environ())
	if out, _ := list.Output(); !strings.Contains(string(out), "downshift/w001") {
		t.Errorf("branch downshift/w001 was not created")
	}
	args := readFile(t, filepath.Join(shimDir, "claude.args"))
	if !strings.Contains(args, "bypassPermissions") {
		t.Errorf("write route must use the bypass permission mode: %s", args)
	}
	// A bypass worker must not inherit the user's MCP servers: their tools would
	// run without approval.
	if !strings.Contains(args, "--strict-mcp-config") {
		t.Errorf("write route must not load the user's MCP servers: %s", args)
	}
	status := exec.Command("git", "-C", repo, "status", "--porcelain")
	status.Env = GitEnv(os.Environ())
	if out, _ := status.Output(); strings.TrimSpace(string(out)) != "" {
		t.Errorf("the checkout is not clean after a write job:\n%s", out)
	}
}

func TestWriteOutsideAGitRepositoryIsRefused(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell shims")
	}
	shim(t, "claude", writeShim)
	q := New(Routes(nil, ByAgent(WriteLaunchers())), 1, fixedNow)
	if _, err := q.Delegate([]Spec{{Agent: "claude", Dir: t.TempDir(), Task: "x", Budget: time.Minute, Write: true}}); err != nil {
		t.Fatal(err)
	}
	c := q.Collect(context.Background(), 0, true, 10*time.Second)
	if len(c.Deliveries) != 1 || c.Deliveries[0].Outcome != "failed" || !strings.Contains(c.Deliveries[0].Text, "git repository") {
		t.Fatalf("deliveries = %+v", c.Deliveries)
	}
}

func TestWriteIsRefusedBeforeAnythingStartsWhenTheRouteIsOff(t *testing.T) {
	q := New(Routes(ByAgent(ReadOnlyLaunchers()), nil), 1, fixedNow)
	if _, err := q.Delegate([]Spec{{Agent: "claude", Dir: t.TempDir(), Task: "x", Budget: time.Minute, Write: true}}); err != nil {
		t.Fatal(err)
	}
	c := q.Collect(context.Background(), 0, true, 5*time.Second)
	if len(c.Deliveries) != 1 || !strings.Contains(c.Deliveries[0].Text, "DOWNSHIFT_WORKERS=bypass") {
		t.Fatalf("deliveries = %+v", c.Deliveries)
	}
}

func TestCodexWriteUsesTheWorkspaceSandboxAndNeverAsks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell shims")
	}
	dir := shim(t, "codex", `out=""; while [ $# -gt 0 ]; do if [ "$1" = "-o" ]; then out="$2"; fi; shift; done; printf ok > "$out"`)
	repo := makeRepo(t)
	q := New(Routes(nil, ByAgent(WriteLaunchers())), 1, fixedNow)
	if _, err := q.Delegate([]Spec{{Agent: "codex", Dir: repo, Task: "x", Budget: time.Minute, Write: true}}); err != nil {
		t.Fatal(err)
	}
	c := q.Collect(context.Background(), 0, true, 20*time.Second)
	if len(c.Deliveries) != 1 || c.Deliveries[0].Outcome != "done" {
		t.Fatalf("deliveries = %+v", c.Deliveries)
	}
	args := readFile(t, filepath.Join(dir, "codex.args"))
	if !strings.Contains(args, "workspace-write") || !strings.Contains(args, "never") || strings.Contains(args, "read-only") {
		t.Errorf("codex write args = %s", args)
	}
}
