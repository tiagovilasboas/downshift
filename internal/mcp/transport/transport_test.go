// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package transport

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift/internal/mcp/tools"
)

const wait = 10 * time.Second

func testTools() []tools.Tool {
	return []tools.Tool{
		{Name: "echo", Description: "greets", InputSchema: json.RawMessage(`{"type":"object","properties":{"name":{"type":"string"}}}`),
			Handler: func(_ context.Context, args json.RawMessage) (tools.Result, error) {
				var a struct{ Name string }
				if len(args) > 0 {
					if err := json.Unmarshal(args, &a); err != nil {
						return tools.Result{}, errors.New("arguments are not valid JSON")
					}
				}
				return tools.Result{Text: "hi " + a.Name}, nil
			}},
		{Name: "fail", Handler: func(context.Context, json.RawMessage) (tools.Result, error) {
			return tools.Result{}, errors.New("the provider is not installed")
		}},
		{Name: "boom", Handler: func(context.Context, json.RawMessage) (tools.Result, error) { panic("secret task text") }},
	}
}

// wire drives a server through the raw protocol, one JSON line at a time.
type wire struct {
	t      *testing.T
	in     *io.PipeWriter
	lines  chan map[string]any
	served chan error
	cancel context.CancelFunc
}

func start(t *testing.T) *wire {
	t.Helper()
	srv, err := New(Info{Name: "downshift-test", Version: "0.0.1"}, testTools(), nil)
	if err != nil {
		t.Fatal(err)
	}
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	ctx, cancel := context.WithCancel(context.Background())
	w := &wire{t: t, in: inW, lines: make(chan map[string]any, 16), served: make(chan error, 1), cancel: cancel}
	go func() { w.served <- srv.Serve(ctx, inR, outW); inR.Close(); outW.Close() }()
	go func() {
		sc := bufio.NewScanner(outR)
		sc.Buffer(make([]byte, 0, 64<<10), 4<<20)
		for sc.Scan() {
			var m map[string]any
			if err := json.Unmarshal(sc.Bytes(), &m); err != nil {
				t.Errorf("stdout carried a line that is not one JSON message: %q", sc.Text())
				continue
			}
			w.lines <- m
		}
		close(w.lines)
	}()
	t.Cleanup(func() { cancel(); inW.Close() })
	return w
}

func (w *wire) send(line string) {
	w.t.Helper()
	if _, err := io.WriteString(w.in, line+"\n"); err != nil {
		w.t.Fatalf("send: %v", err)
	}
}

func (w *wire) recv() map[string]any {
	w.t.Helper()
	select {
	case m, ok := <-w.lines:
		if !ok {
			w.t.Fatal("the server closed its output")
		}
		return m
	case <-time.After(wait):
		w.t.Fatal("no message from the server")
	}
	return nil
}

func result(t *testing.T, m map[string]any) map[string]any {
	t.Helper()
	r, ok := m["result"].(map[string]any)
	if !ok {
		t.Fatalf("not a result: %v", m)
	}
	return r
}

func text(t *testing.T, m map[string]any) string {
	t.Helper()
	c := result(t, m)["content"].([]any)
	return c[0].(map[string]any)["text"].(string)
}

func isError(m map[string]any) bool { b, _ := result0(m)["isError"].(bool); return b }

func result0(m map[string]any) map[string]any { r, _ := m["result"].(map[string]any); return r }

const modernMeta = `"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientCapabilities":{}}`

func TestLegacyEraHandshakeListAndCall(t *testing.T) {
	w := start(t)
	w.send(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"c","version":"0"}}}`)
	init := result(t, w.recv())
	if init["protocolVersion"] != "2025-11-25" || init["serverInfo"].(map[string]any)["name"] != "downshift-test" {
		t.Fatalf("initialize result = %v", init)
	}
	w.send(`{"jsonrpc":"2.0","method":"notifications/initialized"}`)
	w.send(`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`)
	listed := result(t, w.recv())["tools"].([]any)
	var names []string
	for _, x := range listed {
		tool := x.(map[string]any)
		names = append(names, tool["name"].(string))
		if tool["inputSchema"].(map[string]any)["type"] != "object" {
			t.Errorf("tool %v has no object schema", tool["name"])
		}
	}
	slices.Sort(names)
	if !slices.Equal(names, []string{"boom", "echo", "fail"}) {
		t.Fatalf("tools = %v", names)
	}
	w.send(`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"echo","arguments":{"name":"Ana"}}}`)
	if got := text(t, w.recv()); got != "hi Ana" {
		t.Fatalf("echo = %q", got)
	}
}

func TestModernEraDiscoverAndCall(t *testing.T) {
	w := start(t)
	w.send(`{"jsonrpc":"2.0","id":1,"method":"server/discover","params":{` + modernMeta + `}}`)
	d := result(t, w.recv())
	versions, _ := d["supportedVersions"].([]any)
	if !slices.Contains(versions, any("2026-07-28")) || !slices.Contains(versions, any("2025-11-25")) {
		t.Fatalf("supportedVersions = %v", versions)
	}
	if d["resultType"] != "complete" {
		t.Fatalf("resultType = %v", d["resultType"])
	}
	w.send(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"echo","arguments":{"name":"Ana"},` + modernMeta + `}}`)
	m := w.recv()
	if text(t, m) != "hi Ana" || result(t, m)["resultType"] != "complete" {
		t.Fatalf("modern call = %v", m)
	}
}

func TestHandlerErrorIsAFailedResultNotAProtocolError(t *testing.T) {
	w := start(t)
	w.send(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"fail","arguments":{},` + modernMeta + `}}`)
	m := w.recv()
	if _, hasErr := m["error"]; hasErr || !isError(m) || text(t, m) != "the provider is not installed" {
		t.Fatalf("a handler error must be a failed result: %v", m)
	}
}

func TestPanicIsAFailedResultWithoutItsMessageAndTheServerKeepsServing(t *testing.T) {
	w := start(t)
	w.send(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"boom","arguments":{},` + modernMeta + `}}`)
	m := w.recv()
	if !isError(m) || strings.Contains(text(t, m), "secret task text") {
		t.Fatalf("a panic must not leak its message: %v", m)
	}
	w.send(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"echo","arguments":{"name":"B"},` + modernMeta + `}}`)
	if got := text(t, w.recv()); got != "hi B" {
		t.Fatalf("the server stopped serving after a panic: %q", got)
	}
}

func TestUnknownToolIsAProtocolError(t *testing.T) {
	w := start(t)
	w.send(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"nope","arguments":{},` + modernMeta + `}}`)
	if m := w.recv(); m["error"] == nil {
		t.Fatalf("an unknown tool should be an error: %v", m)
	}
}

func TestNewlinesInTextStayInsideOneMessage(t *testing.T) {
	w := start(t)
	w.send(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"echo","arguments":{"name":"a\nb"},` + modernMeta + `}}`)
	if got := text(t, w.recv()); got != "hi a\nb" {
		t.Fatalf("text = %q", got)
	}
}

func TestAnOversizedMessageDoesNotHangTheServer(t *testing.T) {
	w := start(t)
	big := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"echo","arguments":{"name":"` + strings.Repeat("x", 2*MaxMessageBytes) + `"}}}`
	go func() { _, _ = io.WriteString(w.in, big+"\n") }()
	select {
	case <-w.served:
	case <-time.After(wait):
		t.Fatal("the server neither answered nor ended after a message over the cap")
	}
}

// The library treats a line that is not valid JSON as a broken connection and
// ends the session; the protocol is stateless, so the client starts a new
// process. Pinning it here means a library upgrade that changes it shows up.
func TestAMalformedLineEndsTheSession(t *testing.T) {
	w := start(t)
	go func() { _, _ = io.WriteString(w.in, "this is not json\n") }()
	select {
	case <-w.served:
	case <-time.After(wait):
		t.Fatal("the session did not end after a malformed line")
	}
}

func TestServeEndsWhenTheContextEnds(t *testing.T) {
	w := start(t)
	w.cancel()
	select {
	case <-w.served:
	case <-time.After(wait):
		t.Fatal("Serve did not return after its context ended")
	}
}

func TestServeEndsWhenTheInputCloses(t *testing.T) {
	w := start(t)
	w.in.Close()
	select {
	case <-w.served:
	case <-time.After(wait):
		t.Fatal("Serve did not return after stdin closed")
	}
}

func TestNewRejectsAnInvalidToolSet(t *testing.T) {
	if _, err := New(Info{Name: "x"}, []tools.Tool{{Name: "a"}}, nil); err == nil {
		t.Fatal("a tool without a handler was accepted")
	}
}
