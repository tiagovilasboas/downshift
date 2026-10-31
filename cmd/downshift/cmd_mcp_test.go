// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const mcpWait = 15 * time.Second

// mcpSession runs the real server (real adapters, real catalog) over pipes and
// speaks the protocol to it, one JSON line at a time.
type mcpSession struct {
	t      *testing.T
	in     *io.PipeWriter
	lines  chan map[string]any
	served chan int
	errOut *bytes.Buffer
}

func startMcp(t *testing.T, root string) *mcpSession {
	t.Helper()
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	s := &mcpSession{t: t, in: inW, lines: make(chan map[string]any, 16), served: make(chan int, 1), errOut: &bytes.Buffer{}}
	ctx, cancel := context.WithCancel(context.Background())
	go func() { s.served <- serveMcp(ctx, cmdCat, root, inR, outW, s.errOut); inR.Close(); outW.Close() }()
	go func() {
		sc := bufio.NewScanner(outR)
		sc.Buffer(make([]byte, 0, 64<<10), 4<<20)
		for sc.Scan() {
			var m map[string]any
			if err := json.Unmarshal(sc.Bytes(), &m); err != nil {
				t.Errorf("stdout carried a line that is not one JSON message: %q", sc.Text())
				continue
			}
			s.lines <- m
		}
		close(s.lines)
	}()
	t.Cleanup(func() { cancel(); inW.Close() })
	return s
}

func (s *mcpSession) call(id int, tool, args string) map[string]any {
	s.t.Helper()
	msg := `{"jsonrpc":"2.0","id":` + itoa(id) + `,"method":"tools/call","params":{"name":"` + tool + `","arguments":` + args +
		`,"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientCapabilities":{}}}}`
	if _, err := io.WriteString(s.in, msg+"\n"); err != nil {
		s.t.Fatalf("send: %v", err)
	}
	select {
	case m, ok := <-s.lines:
		if !ok {
			s.t.Fatal("the server closed its output")
		}
		return m
	case <-time.After(mcpWait):
		s.t.Fatal("no answer from the server")
	}
	return nil
}

func itoa(n int) string { b, _ := json.Marshal(n); return string(b) }

// payload decodes the JSON object a tool returns in its text content.
func payload(t *testing.T, m map[string]any) (map[string]any, bool) {
	t.Helper()
	res, _ := m["result"].(map[string]any)
	if res == nil {
		t.Fatalf("not a result: %v", m)
	}
	failed, _ := res["isError"].(bool)
	text := res["content"].([]any)[0].(map[string]any)["text"].(string)
	var out map[string]any
	if json.Unmarshal([]byte(text), &out) != nil {
		return map[string]any{"text": text}, failed
	}
	return out, failed
}

func mcpState(t *testing.T) string {
	t.Helper()
	dir := availableState(t)
	root := filepath.Join(dir, "project")
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestMcpServerListsItsTools(t *testing.T) {
	s := startMcp(t, mcpState(t))
	if _, err := io.WriteString(s.in, `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientCapabilities":{}}}}`+"\n"); err != nil {
		t.Fatal(err)
	}
	m := <-s.lines
	var names []string
	for _, x := range m["result"].(map[string]any)["tools"].([]any) {
		names = append(names, x.(map[string]any)["name"].(string))
	}
	if strings.Join(sortedCopy(names), ",") != "apply,cancel,collect,delegate,diff,playbook,providers,status" {
		t.Fatalf("tools = %v", names)
	}
}

func sortedCopy(in []string) []string {
	out := append([]string(nil), in...)
	for i := range out {
		for j := i + 1; j < len(out); j++ {
			if out[j] < out[i] {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}

func TestMcpProvidersReturnsTheDetectedModelsAndAStrainBlock(t *testing.T) {
	root := mcpState(t)
	writeFile(t, filepath.Join(filepath.Dir(root), "session-models.json"), `{"codex":["file-1","file-2"]}`)
	s := startMcp(t, root)
	out, failed := payload(t, s.call(1, "providers", `{"harness":"codex"}`))
	if failed {
		t.Fatalf("providers failed: %v", out)
	}
	data := out["data"].(map[string]any)
	models := data["models"].([]any)
	if len(models) != 2 || models[0].(map[string]any)["source"] != "file" {
		t.Fatalf("models = %v", models)
	}
	if _, ok := out["strain"]; !ok {
		t.Fatalf("every result must carry a strain block: %v", out)
	}
}

func TestMcpProvidersRefusesAnUnknownHarnessWithTheValidOnes(t *testing.T) {
	s := startMcp(t, mcpState(t))
	out, failed := payload(t, s.call(1, "providers", `{"harness":"nope"}`))
	if !failed || !strings.Contains(out["text"].(string), "claude-code") {
		t.Fatalf("expected a failed result naming the valid harnesses: %v", out)
	}
}

func TestMcpPlaybookListsAndReadsTheShippedSets(t *testing.T) {
	s := startMcp(t, mcpState(t))
	out, failed := payload(t, s.call(1, "playbook", `{}`))
	if failed || !strings.Contains(toJSON(out), "Backend") {
		t.Fatalf("list = %v (failed=%v)", out, failed)
	}
	out, failed = payload(t, s.call(2, "playbook", `{"name":"backend"}`))
	if failed || !strings.Contains(toJSON(out), "Validate inputs") {
		t.Fatalf("read = %v (failed=%v)", out, failed)
	}
	if _, failed := payload(t, s.call(3, "playbook", `{"name":"mobile"}`)); !failed {
		t.Fatal("an unknown playbook should be a failed result")
	}
}

func TestMcpApplyKeepsAnUnknownModelAndNeverTakesTaskText(t *testing.T) {
	s := startMcp(t, mcpState(t))
	out, failed := payload(t, s.call(1, "apply", `{"harness":"claude-code","model":"no-such-model"}`))
	if failed {
		t.Fatalf("apply failed: %v", out)
	}
	if applied, _ := out["data"].(map[string]any)["applied"].(bool); applied {
		t.Fatalf("an unknown model must be kept, not moved: %v", out)
	}
	if _, failed := payload(t, s.call(2, "apply", `{"harness":"claude-code","prompt":"secret task"}`)); !failed {
		t.Fatal("a prompt field must be refused")
	}
}

func TestMcpApplyRefusesADirectoryOutsideTheProject(t *testing.T) {
	root := mcpState(t)
	outside := t.TempDir()
	s := startMcp(t, root)
	out, failed := payload(t, s.call(1, "apply", `{"harness":"claude-code","dir":"`+filepath.ToSlash(outside)+`"}`))
	if !failed || strings.Contains(out["text"].(string), outside) {
		t.Fatalf("expected a refusal that names no path: %v", out)
	}
	if _, failed := payload(t, s.call(2, "apply", `{"harness":"claude-code","dir":"`+filepath.ToSlash(root)+`"}`)); failed {
		t.Fatal("the project directory itself should be accepted")
	}
}

func TestMcpStatusReportsTheServerAndEveryHarness(t *testing.T) {
	s := startMcp(t, mcpState(t))
	out, failed := payload(t, s.call(1, "status", `{}`))
	if failed {
		t.Fatalf("status failed: %v", out)
	}
	data := out["data"].(map[string]any)
	if data["server"] != "downshift" {
		t.Fatalf("status data = %v", data)
	}
}

func TestMcpServeKeepsLogsOffStandardOutput(t *testing.T) {
	s := startMcp(t, mcpState(t))
	s.call(1, "status", `{}`)
	// startMcp fails the test if stdout carries anything but one JSON message.
	if strings.Contains(s.errOut.String(), "\x00") {
		t.Fatal("binary noise on stderr")
	}
}

func TestMcpUsageErrors(t *testing.T) {
	var out, errb bytes.Buffer
	if code := runMcp(nil, cmdCat, &out, &errb); code != 2 || !strings.Contains(errb.String(), "usage") {
		t.Fatalf("no subcommand: exit %d, %q", code, errb.String())
	}
	if code := runMcp([]string{"serve", "--x"}, cmdCat, &out, &errb); code != 2 {
		t.Fatalf("extra argument: exit %d", code)
	}
	if out.Len() != 0 {
		t.Fatalf("usage errors must not write to stdout: %q", out.String())
	}
}

func toJSON(v any) string { b, _ := json.Marshal(v); return string(b) }

func TestMcpApplyInABroadRootAcceptsOnlyTheRootItself(t *testing.T) {
	home := availableState(t) // sets HOME to a temp dir
	sub := filepath.Join(home, "project")
	if err := os.MkdirAll(sub, 0o700); err != nil {
		t.Fatal(err)
	}
	s := startMcp(t, home)
	if _, failed := payload(t, s.call(1, "apply", `{"harness":"claude-code","dir":"`+filepath.ToSlash(sub)+`"}`)); !failed {
		t.Fatal("under a home-directory root, a nested directory must be refused")
	}
	if _, failed := payload(t, s.call(2, "apply", `{"harness":"claude-code"}`)); failed {
		t.Fatal("the root itself must be accepted")
	}
}

func TestMcpApplyRefusesAnUnknownTierAndEffort(t *testing.T) {
	s := startMcp(t, mcpState(t))
	for i, args := range []string{`{"harness":"claude-code","tier":"huge"}`, `{"harness":"claude-code","effort":"zz"}`} {
		out, failed := payload(t, s.call(i+1, "apply", args))
		if !failed || strings.Contains(out["text"].(string), "zz") {
			t.Fatalf("args %s: want a failed result that does not echo the value, got %v", args, out)
		}
	}
	if _, failed := payload(t, s.call(9, "apply", `{"harness":"claude-code","effort":"xhigh"}`)); failed {
		t.Fatal("xhigh is a catalog effort and must be accepted")
	}
}
