// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// The tools/call half of the MCP probe (doctor_mcp.go). Listing tools proves
// the server starts and answers; calling one proves a tool returns a result.

// mcpCallID is the request id of the tools/call; mcpInitID and mcpListID are in
// doctor_mcp.go.
const mcpCallID = 3

// mcpCallTool is the tool the probe calls. `playbook` with no arguments only
// reads the shipped playbooks. `providers` is not used: it needs a harness
// argument and is the one information tool that can store a hint.
const mcpCallTool = "playbook"

// mcpProbeResult is what one probe learned from the server.
type mcpProbeResult struct {
	// Tools is the number of tools the server listed.
	Tools int
	// CallErr is why the tools/call did not return a valid tool result; nil
	// when it did. The server may list tools and still fail to call one.
	CallErr error
}

// mcpCallRequest is the tools/call the probe sends after tools/list.
func mcpCallRequest() map[string]any {
	return map[string]any{"jsonrpc": "2.0", "id": mcpCallID, "method": "tools/call",
		"params": map[string]any{"name": mcpCallTool, "arguments": map[string]any{}}}
}

// mcpResponse is the slice of a JSON-RPC message the probe reads: the id that
// matches a reply to its request, and the tools/list and tools/call results.
type mcpResponse struct {
	ID     json.Number `json:"id"`
	Method string      `json:"method"`
	Result struct {
		Tools   []json.RawMessage `json:"tools"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	} `json:"result"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// toolCallErr reports why the reply is not a valid tool result: a protocol
// error, an error result, or no text content. A valid result carries at least
// one non-empty text content item and is not flagged isError.
func (m mcpResponse) toolCallErr() error {
	if m.Error != nil {
		return fmt.Errorf("tools/call %s error: %s", mcpCallTool, m.Error.Message)
	}
	var text string
	for _, c := range m.Result.Content {
		if c.Type == "text" && strings.TrimSpace(c.Text) != "" {
			text = strings.TrimSpace(c.Text)
			break
		}
	}
	switch {
	case m.Result.IsError:
		return fmt.Errorf("%s returned an error result: %s", mcpCallTool, firstLine(text))
	case text == "":
		return fmt.Errorf("%s returned no text content", mcpCallTool)
	}
	return nil
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	if len(s) > 120 {
		s = s[:120]
	}
	return s
}

// mcpLine is one decoded line of server output, or the end of the stream.
type mcpLine struct {
	msg mcpResponse
	err error // set on the last item: the scanner error or end of output
}

// readMcpReplies reads newline-delimited replies until both the tools/list
// (id mcpListID) and the tools/call (id mcpCallID) replies have arrived. A
// missing tools/list reply is an error. A missing or invalid tools/call reply
// is reported in CallErr, so the tools row still shows the server answered.
// It stops on ctx cancellation, end of output or a protocol error.
func readMcpReplies(ctx context.Context, r interface{ Read([]byte) (int, error) }) (mcpProbeResult, error) {
	lines := make(chan mcpLine, 16)
	go func() {
		sc := bufio.NewScanner(r)
		sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
		for sc.Scan() {
			var m mcpResponse
			if json.Unmarshal(sc.Bytes(), &m) != nil || m.Method != "" {
				continue // a log line, a notification or a server request
			}
			select {
			case lines <- mcpLine{msg: m}:
			default: // the probe only needs two replies; drop a flood
			}
		}
		err := sc.Err()
		if err == nil {
			err = errors.New("the server closed its output")
		}
		lines <- mcpLine{err: err}
	}()

	var res mcpProbeResult
	listed, called := false, false
	// finish turns the reason the stream stopped into the outcome for the
	// replies that never came.
	finish := func(why error) (mcpProbeResult, error) {
		if !listed {
			return res, fmt.Errorf("%w before listing its tools", why)
		}
		res.CallErr = fmt.Errorf("%w before answering tools/call %s", why, mcpCallTool)
		return res, nil
	}
	for !listed || !called {
		select {
		case <-ctx.Done():
			return finish(ctx.Err())
		case l := <-lines:
			if l.err != nil {
				return finish(l.err)
			}
			switch l.msg.ID.String() {
			case fmt.Sprint(mcpListID):
				if l.msg.Error != nil {
					return res, fmt.Errorf("tools/list error: %s", l.msg.Error.Message)
				}
				res.Tools, listed = len(l.msg.Result.Tools), true
			case fmt.Sprint(mcpCallID):
				res.CallErr, called = l.msg.toolCallErr(), true
			}
		}
	}
	return res, nil
}
