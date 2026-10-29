// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package workers

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// shim writes a fake CLI named name into a fresh directory placed first on
// PATH. It records its arguments and its standard input in the directory, then
// runs body. The shims are shell scripts, so these tests skip on Windows.
func shim(t *testing.T, name, body string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("shell shims")
	}
	dir := t.TempDir()
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > " + filepath.Join(dir, name+".args") + "\ncat > " + filepath.Join(dir, name+".stdin") + "\n" + body + "\n"
	if err := os.WriteFile(filepath.Join(dir, name), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return dir
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestClaudeRunsInPlanModeWithThePromptOnStdin(t *testing.T) {
	dir := shim(t, "claude", `echo '{"type":"result","subtype":"success","is_error":false,"result":"the answer"}'`)
	work := t.TempDir()
	proc, err := ReadOnlyLaunchers()["claude"].Launch(Spec{Agent: "claude", Dir: work, Task: "read a.txt", Budget: time.Minute, Model: "claude-haiku-5-5"})
	if err != nil {
		t.Fatal(err)
	}
	if err := proc.Wait(); err != nil {
		t.Fatalf("wait: %v", err)
	}
	args := readFile(t, filepath.Join(dir, "claude.args"))
	for _, want := range []string{"-p", "json", "plan", "--no-session-persistence", "claude-haiku-5-5"} {
		if !strings.Contains(args, want) {
			t.Errorf("args lack %q: %s", want, args)
		}
	}
	if strings.Contains(args, "bypassPermissions") || strings.Contains(args, "acceptEdits") {
		t.Errorf("a write-capable mode reached the read-only launch: %s", args)
	}
	if got := readFile(t, filepath.Join(dir, "claude.stdin")); got != "read a.txt" {
		t.Errorf("prompt on stdin = %q", got)
	}
	if proc.Report() != "the answer" {
		t.Errorf("report = %q", proc.Report())
	}
}

func TestClaudeErrorResultIsAFailure(t *testing.T) {
	shim(t, "claude", `echo '{"type":"result","subtype":"error_max_turns","is_error":true,"result":"stopped"}'`)
	proc, err := ReadOnlyLaunchers()["claude"].Launch(Spec{Agent: "claude", Dir: t.TempDir(), Task: "x", Budget: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	if err := proc.Wait(); err == nil {
		t.Fatal("an error result must fail the job")
	}
}

func TestCodexRunsInTheReadOnlySandboxAndReadsItsLastMessage(t *testing.T) {
	dir := shim(t, "codex", `out=""; while [ $# -gt 0 ]; do if [ "$1" = "-o" ]; then out="$2"; fi; shift; done; printf 'codex says hi' > "$out"`)
	work := t.TempDir()
	proc, err := ReadOnlyLaunchers()["codex"].Launch(Spec{Agent: "codex", Dir: work, Task: "look", Budget: time.Minute, Effort: "high"})
	if err != nil {
		t.Fatal(err)
	}
	if err := proc.Wait(); err != nil {
		t.Fatalf("wait: %v", err)
	}
	args := readFile(t, filepath.Join(dir, "codex.args"))
	for _, want := range []string{"exec", "read-only", "--ephemeral", "model_reasoning_effort=high"} {
		if !strings.Contains(args, want) {
			t.Errorf("args lack %q: %s", want, args)
		}
	}
	if strings.Contains(args, "workspace-write") || strings.Contains(args, "danger-full-access") {
		t.Errorf("a writable sandbox reached the launch: %s", args)
	}
	if proc.Report() != "codex says hi" {
		t.Errorf("report = %q", proc.Report())
	}
}

func TestCodexReportFileIsRemovedAfterTheWait(t *testing.T) {
	shim(t, "codex", `out=""; while [ $# -gt 0 ]; do if [ "$1" = "-o" ]; then out="$2"; fi; shift; done; printf done > "$out"; echo "$out" > "`+os.TempDir()+`/downshift-last-report-path"`)
	proc, err := ReadOnlyLaunchers()["codex"].Launch(Spec{Agent: "codex", Dir: t.TempDir(), Task: "x", Budget: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	if err := proc.Wait(); err != nil {
		t.Fatal(err)
	}
	path := strings.TrimSpace(readFile(t, filepath.Join(os.TempDir(), "downshift-last-report-path")))
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("the report file %s was left behind", path)
	}
}

func TestPlannedValuesThatWouldBecomeFlagsAreRefused(t *testing.T) {
	shim(t, "claude", `echo '{"subtype":"success","result":""}'`)
	for _, spec := range []Spec{
		{Agent: "claude", Dir: t.TempDir(), Task: "x", Budget: time.Minute, Model: "--dangerously-skip-permissions"},
		{Agent: "claude", Dir: t.TempDir(), Task: "x", Budget: time.Minute, Effort: "high max"},
		{Agent: "claude", Dir: filepath.Join(t.TempDir(), "missing"), Task: "x", Budget: time.Minute},
		{Agent: "claude", Dir: "", Task: "x", Budget: time.Minute},
	} {
		if _, err := ReadOnlyLaunchers()["claude"].Launch(spec); err == nil {
			t.Errorf("spec %+v was accepted", spec)
		}
	}
}

func TestAMissingProgramIsAStartFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("PATH shims")
	}
	t.Setenv("PATH", t.TempDir())
	if _, err := ReadOnlyLaunchers()["claude"].Launch(Spec{Agent: "claude", Dir: t.TempDir(), Task: "x", Budget: time.Minute}); err == nil || !strings.Contains(err.Error(), "not on PATH") {
		t.Fatalf("err = %v, want not on PATH", err)
	}
}

func TestQueueRunsARealShimToADelivery(t *testing.T) {
	shim(t, "claude", `echo '{"subtype":"success","is_error":false,"result":"end to end"}'`)
	q := New(ByAgent(ReadOnlyLaunchers()), 1, fixedNow)
	if _, err := q.Delegate([]Spec{{Agent: "claude", Dir: t.TempDir(), Task: "go", Budget: time.Minute}}); err != nil {
		t.Fatal(err)
	}
	c := q.Collect(context.Background(), 0, true, 10*time.Second)
	if len(c.Deliveries) != 1 || c.Deliveries[0].Outcome != "done" || c.Deliveries[0].Text != "end to end" {
		t.Fatalf("deliveries = %+v", c.Deliveries)
	}
}

func TestByAgentRefusesAnAgentWithoutALauncher(t *testing.T) {
	if _, err := ByAgent(ReadOnlyLaunchers()).Launch(Spec{Agent: "kiro", Dir: t.TempDir(), Task: "x", Budget: time.Minute}); err != ErrNoVerifiedMode {
		t.Fatalf("err = %v, want ErrNoVerifiedMode", err)
	}
}
