// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/adapters/claudecode"
	"github.com/tiagovilasboas/downshift-ai/internal/core"
	"github.com/tiagovilasboas/downshift-ai/internal/execwrap"
	"github.com/tiagovilasboas/downshift-ai/internal/hookport"
	"github.com/tiagovilasboas/downshift-ai/internal/postrun"
	"github.com/tiagovilasboas/downshift-ai/internal/testenv"
)

// These tests drive the shared honor runner (runHonor) with real-shaped
// PostToolUse payloads: the Bash shape is the one the hooks reference
// documents ({stdout, stderr, interrupted, isImage}); Grep and Glob carry the
// fields their results hold (content, filenames) plus counts that must come
// back untouched.

const postSession = "0b6a2f3e-1111-4222-8333-444455556666"

// postRunEnv isolates state (CCR store, exec log, config) and pins the
// Compactor on. onPath false makes the pre-run rewrite unavailable, so a
// plain allowlisted command reaches the post-run path too.
func postRunEnv(t *testing.T, onPath bool) string {
	t.Helper()
	dir := t.TempDir()
	testenv.SetHome(t, dir)
	t.Setenv("DOWNSHIFT_STATE_DIR", filepath.Join(dir, ".downshift"))
	t.Setenv("DOWNSHIFT_EVENT_LOG", filepath.Join(dir, "events.jsonl"))
	for _, k := range []string{postrun.Env, "DOWNSHIFT_CCR", "DOWNSHIFT_BALANCED", execEnvKillSwitch} {
		t.Setenv(k, "")
	}
	withExec(t, true, nil, onPath)
	return filepath.Join(dir, ".downshift")
}

func postPayload(t *testing.T, tool string, input, resp map[string]any) []byte {
	t.Helper()
	raw, err := json.Marshal(map[string]any{
		"session_id": postSession, "transcript_path": "/x/" + postSession + ".jsonl", "cwd": "/tmp",
		"permission_mode": "default", "hook_event_name": "PostToolUse", "tool_name": tool,
		"tool_input": input, "tool_response": resp, "tool_use_id": "toolu_01ABC", "duration_ms": 1200,
	})
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func bashPost(t *testing.T, command, stdout, stderr string) []byte {
	return postPayload(t, "Bash", map[string]any{"command": command, "description": "run"},
		map[string]any{"stdout": stdout, "stderr": stderr, "interrupted": false, "isImage": false, "noOutputExpected": false})
}

// verboseGoTest is `go test -v` output of n passing tests.
func verboseGoTest(n int) string {
	var b strings.Builder
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, "=== RUN   TestThing%d\n--- PASS: TestThing%d (0.00s)\n", i, i)
	}
	b.WriteString("PASS\nok  \texample.com/x/y\t0.512s\n")
	return b.String()
}

// honorAnswer runs the shared runner on payload for port and returns the
// updated tool output, or nil when the answer carries none.
func honorAnswer(t *testing.T, port hookport.Port, payload []byte) (answer string, updated map[string]any) {
	t.Helper()
	answer = captureStdout(func() {
		if rc := runHonor(bytes.NewReader(payload), port, cmdCat); rc != 0 {
			t.Errorf("runHonor exit %d, want 0", rc)
		}
	})
	var out struct {
		HookSpecificOutput struct {
			HookEventName     string         `json:"hookEventName"`
			UpdatedToolOutput map[string]any `json:"updatedToolOutput"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal([]byte(answer), &out); err != nil {
		t.Fatalf("answer is not JSON: %q", answer)
	}
	if out.HookSpecificOutput.UpdatedToolOutput != nil && out.HookSpecificOutput.HookEventName != "PostToolUse" {
		t.Errorf("hookEventName = %q, want PostToolUse", out.HookSpecificOutput.HookEventName)
	}
	return answer, out.HookSpecificOutput.UpdatedToolOutput
}

func execRows(t *testing.T, state string) []execwrap.Event {
	t.Helper()
	rows, err := execwrap.ReadEvents(filepath.Join(state, execwrap.EventsFileName))
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	return rows
}

func TestPostRun_PipedCommandCompactedAndCounted(t *testing.T) {
	state := postRunEnv(t, true)
	stdout := verboseGoTest(800)
	for _, command := range []string{
		"go test -v ./... 2>&1 | grep -v '^$'",
		"go test -v ./... > /tmp/log.txt 2>&1; cat /tmp/log.txt",
	} {
		_, got := honorAnswer(t, portFor("claude-code"), bashPost(t, command, stdout, ""))
		if got == nil {
			t.Fatalf("%q: no updatedToolOutput", command)
		}
		out, _ := got["stdout"].(string)
		if len(out) >= len(stdout)/4 || !strings.Contains(out, "downshift retrieve ") {
			t.Errorf("%q: stdout %d bytes from %d, want a balanced cut naming the retrieve id", command, len(out), len(stdout))
		}
		for k, want := range map[string]any{"stderr": "", "interrupted": false, "isImage": false, "noOutputExpected": false} {
			if got[k] != want {
				t.Errorf("%q: sibling %s = %v, want %v", command, k, got[k], want)
			}
		}
	}
	rows := execRows(t, state)
	if len(rows) != 2 {
		t.Fatalf("exec rows = %d, want 2", len(rows))
	}
	for _, r := range rows {
		if r.Entry != postrun.Entry || r.Harness != "claude-code" || !r.Compacted || r.Kept == "" || r.Session == "" || r.EmittedBytes >= r.RawBytes {
			t.Errorf("row %+v: want a compacted post_tool_use row with a kept id and a hashed session", r)
		}
		if strings.Contains(r.Session, postSession) {
			t.Error("the raw session id reached the exec log")
		}
		if r.Inner != "go-test" || r.Kind == "" {
			t.Errorf("row inner %q kind %q, want the compound command's go-test and its kind", r.Inner, r.Kind)
		}
	}
}

func TestPostRun_WrappedCommandUntouched(t *testing.T) {
	state := postRunEnv(t, true)
	stdout := verboseGoTest(800)
	for _, command := range []string{
		"'" + testExe + "' compact --session=abc --harness=claude-code -- go test -v ./...", // as rewritten
		"go test -v ./...", // as written, when the pre-run hook wrapped it
	} {
		if answer, got := honorAnswer(t, portFor("claude-code"), bashPost(t, command, stdout, "")); got != nil || strings.TrimSpace(answer) != "{}" {
			t.Errorf("%q: answer %q, want {} (the wrapper already compacted it)", command, answer)
		}
	}
	marked := "ok\n" + execwrap.MarkerWithID(9000, 3, "0123456789ab")
	if _, got := honorAnswer(t, portFor("claude-code"), bashPost(t, "make test | cat", marked, "")); got != nil {
		t.Error("an output ending in the wrapper's marker was compacted again")
	}
	if rows := execRows(t, state); len(rows) != 0 {
		t.Errorf("exec rows = %d, want 0: a wrapped run is logged by the wrapper only", len(rows))
	}
}

// With the pre-run rewrite unavailable, a plain allowlisted command gets the
// same family compressor the wrapper uses, lossless and below the threshold.
func TestPostRun_PlainFamilyUsesWrapperCompressor(t *testing.T) {
	postRunEnv(t, false)
	stdout := verboseGoTest(40)
	_, got := honorAnswer(t, portFor("claude-code"), bashPost(t, "go test -v ./...", stdout, ""))
	if got == nil {
		t.Fatal("no updatedToolOutput for a clean go test -v run")
	}
	if out := got["stdout"].(string); !strings.Contains(out, "; downshift retrieve ") || len(out) >= len(stdout) {
		t.Errorf("stdout = %q, want the go test summary with its marker", out)
	}
}

func TestPostRun_SmallOrUnknownOutputUnchanged(t *testing.T) {
	state := postRunEnv(t, true)
	answer, got := honorAnswer(t, portFor("claude-code"), bashPost(t, "ls -la | sort", "a\nb\n", ""))
	if got != nil || strings.TrimSpace(answer) != "{}" {
		t.Errorf("small output: answer %q, want {}", answer)
	}
	rows := execRows(t, state)
	if len(rows) != 1 || rows[0].Compacted || rows[0].Passthrough == "" || rows[0].Entry != postrun.Entry {
		t.Errorf("rows = %+v, want one raw row with a passthrough reason", rows)
	}
}

func TestPostRun_LargeStderrCutStdoutKept(t *testing.T) {
	postRunEnv(t, true)
	var errOut strings.Builder
	for i := 0; i < 900; i++ {
		fmt.Fprintf(&errOut, "warning: unused variable x%d in pkg/m%d.c\n", i, i)
	}
	errOut.WriteString("error: build failed in pkg/m7.c\n")
	_, got := honorAnswer(t, portFor("claude-code"), bashPost(t, "make all 2>&1 >/dev/null | tee b.log", "built\n", errOut.String()))
	if got == nil {
		t.Fatal("no updatedToolOutput for a large stderr")
	}
	if got["stdout"] != "built\n" {
		t.Errorf("stdout = %q, want it unchanged", got["stdout"])
	}
	if e := got["stderr"].(string); len(e) >= errOut.Len() || !strings.Contains(e, "error: build failed in pkg/m7.c") {
		t.Errorf("stderr not cut, or the failure line was lost")
	}
}

func TestPostRun_GrepAndGlobKeepShape(t *testing.T) {
	state := postRunEnv(t, true)
	var lines []string
	for f := 0; f < 100; f++ {
		for n := 0; n < 20; n++ {
			lines = append(lines, fmt.Sprintf("internal/pkg%02d/file%02d.go:%d:\tif err := doSomething(ctx, value%d); err != nil {", f, f, n*7+3, n))
		}
	}
	content := strings.Join(lines, "\n")
	grep := postPayload(t, "Grep", map[string]any{"pattern": "err != nil", "output_mode": "content", "-n": true},
		map[string]any{"mode": "content", "numFiles": 0, "filenames": []any{}, "content": content, "numLines": len(lines)})
	answer, got := honorAnswer(t, portFor("claude-code"), grep)
	if got == nil {
		t.Fatal("a 2,000-match Grep was not cut")
	}
	if len(answer) >= execwrap.BalanceThreshold {
		t.Errorf("answer is %d bytes, want under %d", len(answer), execwrap.BalanceThreshold)
	}
	if got["mode"] != "content" || got["numLines"] != float64(len(lines)) || got["numFiles"] != float64(0) {
		t.Errorf("counts or mode changed: %v %v %v", got["mode"], got["numLines"], got["numFiles"])
	}
	if c := got["content"].(string); !strings.Contains(c, "more matches in") || !strings.Contains(c, "downshift retrieve ") {
		t.Errorf("content lacks the count of the rest or the retrieve id")
	}

	var paths []any
	for i := 0; i < 1500; i++ {
		paths = append(paths, fmt.Sprintf("/home/user/project/internal/module%03d/sub/file_%04d_test.go", i%300, i))
	}
	glob := postPayload(t, "Glob", map[string]any{"pattern": "**/*_test.go"},
		map[string]any{"filenames": paths, "durationMs": 40, "numFiles": len(paths), "truncated": false})
	answer, got = honorAnswer(t, portFor("claude-code"), glob)
	if got == nil || len(answer) >= execwrap.BalanceThreshold {
		t.Fatalf("a 1,500-path Glob was not cut under the threshold (answer %d bytes)", len(answer))
	}
	if got["numFiles"] != float64(len(paths)) || got["truncated"] != false || got["durationMs"] != float64(40) {
		t.Errorf("Glob siblings changed: %v", got)
	}
	names := got["filenames"].([]any)
	if last := names[len(names)-1].(string); !strings.Contains(last, "downshift retrieve ") {
		t.Errorf("last filename entry = %q, want the marker", last)
	}
	for _, r := range execRows(t, state) {
		if r.Family != postrun.FamilySearchTool || !r.Compacted {
			t.Errorf("row %+v, want a compacted search-tool row", r)
		}
	}
}

// Only the fields a result carries are written: a Glob result has no
// content, so none is added; a Read result is never touched.
func TestPostRun_NoFieldAddedAndReadUntouched(t *testing.T) {
	postRunEnv(t, true)
	big := strings.Repeat("line of a large file that the agent read\n", 2000)
	read := postPayload(t, "Read", map[string]any{"file_path": "/p/big.txt"},
		map[string]any{"type": "text", "file": map[string]any{"filePath": "/p/big.txt", "content": big, "numLines": 2000}})
	if answer, got := honorAnswer(t, portFor("claude-code"), read); got != nil {
		t.Errorf("a Read result was replaced: %q", answer[:80])
	}
	var paths []any
	for i := 0; i < 1500; i++ {
		paths = append(paths, fmt.Sprintf("/p/module%03d/file_%04d.go", i%300, i))
	}
	_, got := honorAnswer(t, portFor("claude-code"), postPayload(t, "Glob", map[string]any{"pattern": "**"}, map[string]any{"filenames": paths}))
	if _, has := got["content"]; has || len(got) != 1 {
		t.Errorf("updated Glob output has fields %v, want only filenames", got)
	}
}

// A result that must not or cannot be replaced fails open with the usual
// answer: garbage, an interrupted run, an image, an unknown filenames shape.
func TestPostRun_FailOpen(t *testing.T) {
	postRunEnv(t, true)
	big := verboseGoTest(800)
	cases := map[string][]byte{
		"garbage":     []byte(`{"tool_name":"Bash","tool_response":`),
		"not json":    []byte("\x00\x01 not a payload"),
		"interrupted": postPayload(t, "Bash", map[string]any{"command": "a | b"}, map[string]any{"stdout": big, "stderr": "", "interrupted": true, "isImage": false}),
		"image":       postPayload(t, "Bash", map[string]any{"command": "a | b"}, map[string]any{"stdout": big, "stderr": "", "interrupted": false, "isImage": true}),
		"odd list":    postPayload(t, "Glob", map[string]any{}, map[string]any{"filenames": []any{1, 2, 3}, "content": big}),
		"string resp": postPayload(t, "Bash", map[string]any{"command": "a | b"}, nil),
	}
	for name, p := range cases {
		if answer, got := honorAnswer(t, portFor("claude-code"), p); got != nil || strings.TrimSpace(answer) != "{}" {
			t.Errorf("%s: answer %q, want {}", name, answer)
		}
	}
}

// The capability is data: the same adapter funcs on a harness that does not
// declare ReplacesToolOutput give no replacement, and the switch turns it off.
func TestPostRun_CapabilityAndSwitch(t *testing.T) {
	postRunEnv(t, true)
	payload := bashPost(t, "go test -v ./... 2>&1 | grep -v '^$'", verboseGoTest(800), "")
	noCap := hookport.Port{
		ID:                "cursor",
		Honor:             func([]byte, string, core.Resolver) (any, string) { return nil, "" },
		ParseToolOutput:   claudecode.ParseToolOutput,
		ReplaceToolOutput: claudecode.ReplaceToolOutput,
	}
	if answer, got := honorAnswer(t, noCap, payload); got != nil || strings.TrimSpace(answer) != "{}" {
		t.Errorf("harness without the capability: answer %q, want {}", answer)
	}
	t.Setenv(postrun.Env, "0")
	if _, got := honorAnswer(t, portFor("claude-code"), payload); got != nil {
		t.Errorf("%s=0 still replaced the output", postrun.Env)
	}
	t.Setenv(postrun.Env, "")
	t.Setenv(execEnvKillSwitch, "0")
	if _, got := honorAnswer(t, portFor("claude-code"), payload); got != nil {
		t.Errorf("%s=0 still replaced the output", execEnvKillSwitch)
	}
}

// Without stored raw output (CCR off) nothing is cut with loss.
func TestPostRun_NoCutWithoutCCR(t *testing.T) {
	postRunEnv(t, true)
	t.Setenv("DOWNSHIFT_CCR", "0")
	if _, got := honorAnswer(t, portFor("claude-code"), bashPost(t, "go test -v ./... | cat -A", verboseGoTest(800), "")); got != nil {
		t.Error("output cut with CCR off")
	}
}
