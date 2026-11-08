// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift-ai/internal/testenv"
)

// reply lines of a server, as readMcpReplies sees them.
const (
	listReply = `{"jsonrpc":"2.0","id":2,"result":{"tools":[{"name":"a"},{"name":"b"}]}}` + "\n"
	okCall    = `{"jsonrpc":"2.0","id":3,"result":{"content":[{"type":"text","text":"{\"playbooks\":[]}"}]}}` + "\n"
)

func readReplies(t *testing.T, out string) (mcpProbeResult, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return readMcpReplies(ctx, strings.NewReader(out))
}

func TestReadMcpRepliesValidToolResult(t *testing.T) {
	// A log line, a notification and a server request around the replies are skipped,
	// and the replies may arrive in either order.
	noise := "not json\n" + `{"jsonrpc":"2.0","method":"notifications/message"}` + "\n" +
		`{"jsonrpc":"2.0","id":3,"method":"ping"}` + "\n"
	for name, out := range map[string]string{
		"list then call": noise + listReply + okCall,
		"call then list": noise + okCall + listReply,
	} {
		res, err := readReplies(t, out)
		if err != nil || res.Tools != 2 || res.CallErr != nil {
			t.Errorf("%s: res = %+v, err = %v, want 2 tools and a valid call", name, res, err)
		}
	}
}

func TestReadMcpRepliesInvalidToolResult(t *testing.T) {
	cases := map[string]struct{ call, want string }{
		"error result": {
			`{"jsonrpc":"2.0","id":3,"result":{"isError":true,"content":[{"type":"text","text":"the playbooks cannot be read\nmore"}]}}`,
			"playbook returned an error result: the playbooks cannot be read",
		},
		"protocol error": {
			`{"jsonrpc":"2.0","id":3,"error":{"code":-32602,"message":"unknown tool"}}`,
			"tools/call playbook error: unknown tool",
		},
		"no content": {
			`{"jsonrpc":"2.0","id":3,"result":{"content":[]}}`,
			"playbook returned no text content",
		},
		"blank text": {
			`{"jsonrpc":"2.0","id":3,"result":{"content":[{"type":"text","text":"  "}]}}`,
			"playbook returned no text content",
		},
		"not text": {
			`{"jsonrpc":"2.0","id":3,"result":{"content":[{"type":"image","text":"x"}]}}`,
			"playbook returned no text content",
		},
	}
	for name, c := range cases {
		res, err := readReplies(t, listReply+c.call+"\n")
		if err != nil || res.Tools != 2 {
			t.Errorf("%s: a bad call must not hide the tool list: res = %+v, err = %v", name, res, err)
			continue
		}
		if res.CallErr == nil || !strings.Contains(res.CallErr.Error(), c.want) {
			t.Errorf("%s: CallErr = %v, want it to contain %q", name, res.CallErr, c.want)
		}
	}
}

func TestReadMcpRepliesServerStopsBeforeTheCall(t *testing.T) {
	res, err := readReplies(t, listReply)
	if err != nil || res.Tools != 2 {
		t.Fatalf("res = %+v, err = %v, want the list to count", res, err)
	}
	if res.CallErr == nil || !strings.Contains(res.CallErr.Error(), "before answering tools/call playbook") {
		t.Errorf("CallErr = %v", res.CallErr)
	}
}

func TestReadMcpRepliesNoListIsAnError(t *testing.T) {
	_, err := readReplies(t, okCall)
	if err == nil || !strings.Contains(err.Error(), "before listing its tools") {
		t.Errorf("err = %v", err)
	}
	_, err = readReplies(t, `{"jsonrpc":"2.0","id":2,"error":{"message":"nope"}}`+"\n")
	if err == nil || !strings.Contains(err.Error(), "tools/list error: nope") {
		t.Errorf("err = %v", err)
	}
}

// A server that lists its tools and then goes quiet must end at the timeout,
// with the list kept and the call reported.
func TestReadMcpRepliesTimesOutOnAMissingCall(t *testing.T) {
	pr, pw := io.Pipe()
	t.Cleanup(func() { pw.Close() })
	go func() { _, _ = io.WriteString(pw, listReply) }()
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	res, err := readMcpReplies(ctx, pr)
	if err != nil || res.Tools != 2 {
		t.Fatalf("res = %+v, err = %v", res, err)
	}
	if !errors.Is(res.CallErr, context.DeadlineExceeded) {
		t.Errorf("CallErr = %v, want the deadline", res.CallErr)
	}
}

func TestMcpCallRequestIsAReadOnlyToolCall(t *testing.T) {
	raw, err := json.Marshal(mcpCallRequest())
	if err != nil {
		t.Fatal(err)
	}
	want := `{"id":3,"jsonrpc":"2.0","method":"tools/call","params":{"arguments":{},"name":"playbook"}}`
	if string(raw) != want {
		t.Errorf("request = %s, want %s", raw, want)
	}
}

// The real server's playbook result passes the probe's validity check, so the
// check and the server cannot drift apart.
func TestRealServerPlaybookResultIsAValidToolResult(t *testing.T) {
	s := startMcp(t, mcpState(t))
	msg := s.call(mcpCallID, mcpCallTool, `{}`)
	raw, err := json.Marshal(msg)
	if err != nil {
		t.Fatal(err)
	}
	var m mcpResponse
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	if err := m.toolCallErr(); err != nil {
		t.Errorf("the real playbook result is not valid for the probe: %v\n%s", err, raw)
	}
}

func TestProbeMcpToolsReportsABadCall(t *testing.T) {
	script := fakeServerWithCall(t, 3, `{"isError":true,"content":[{"type":"text","text":"boom"}]}`)
	ctx, cancel := context.WithTimeout(context.Background(), testenv.Scaled(fakeServerDeadline))
	defer cancel()
	res, err := probeMcpTools(ctx, script)
	if err != nil || res.Tools != 3 {
		t.Fatalf("res = %+v, err = %v", res, err)
	}
	if res.CallErr == nil || !strings.Contains(res.CallErr.Error(), "error result: boom") {
		t.Errorf("CallErr = %v", res.CallErr)
	}
}

func TestProbeMcpToolsServerThatDiesOnTheCall(t *testing.T) {
	script := fakeServerWithCall(t, 3, "")
	ctx, cancel := context.WithTimeout(context.Background(), testenv.Scaled(fakeServerDeadline))
	defer cancel()
	res, err := probeMcpTools(ctx, script)
	if err != nil || res.Tools != 3 {
		t.Fatalf("res = %+v, err = %v", res, err)
	}
	if res.CallErr == nil || !strings.Contains(res.CallErr.Error(), "before answering tools/call") {
		t.Errorf("CallErr = %v", res.CallErr)
	}
}

func TestCheckMcpAnswersReportsTheToolCall(t *testing.T) {
	home, exe := mcpTestEnv(t)
	writeClaudeEntry(t, home, exe, true)

	ok := collectMcp(envFor(home, exe, func(context.Context, string) (mcpProbeResult, error) {
		return mcpProbeResult{Tools: 11}, nil
	}))
	if r := ok["mcp tool call"]; r.status != doctorOK || !strings.Contains(r.detail, "playbook returned a valid tool result") || r.fix != "" {
		t.Errorf("tool call row = %+v", r)
	}

	bad := collectMcp(envFor(home, exe, func(context.Context, string) (mcpProbeResult, error) {
		return mcpProbeResult{Tools: 11, CallErr: errors.New("playbook returned no text content")}, nil
	}))
	if r := bad["mcp answers"]; r.status != doctorOK {
		t.Errorf("a server that lists its tools still answers: %+v", r)
	}
	if r := bad["mcp tool call"]; r.status != doctorWarn || r.fix != mcpSetupFix ||
		!strings.Contains(r.detail, "listed its tools but playbook returned no text content") {
		t.Errorf("tool call row = %+v", r)
	}

	none := collectMcp(envFor(home, exe, func(context.Context, string) (mcpProbeResult, error) {
		return mcpProbeResult{}, errors.New("boom")
	}))
	if _, ok := none["mcp tool call"]; ok {
		t.Error("no tool call row when the server did not answer at all")
	}
}
