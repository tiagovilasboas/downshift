// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package opencode_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/adapters/opencode"
	"github.com/tiagovilasboas/downshift-ai/internal/testenv"
)

const shellChild = "0123456789abcdef0123456789abcdef"

func shellCall(command, dir string) opencode.Request {
	return opencode.Request{
		V:           1,
		Kind:        "shell",
		Harness:     opencode.RequestHarness{ID: "opencode", Version: "2.2.5"},
		Event:       "shell_command",
		Correlation: opencode.RequestRelation{Child: shellChild},
		Command:     command,
		Directory:   dir,

		SessionDirectory: dir,
	}
}

func TestShellCommand_OnlyShellCallsCarryACommand(t *testing.T) {
	route := opencode.Request{V: 1, Kind: "route", Event: "subagent_spawn_intent", Command: "go test ./..."}
	if cmd, isShell := route.ShellCommand(); cmd != "" || isShell {
		t.Fatalf("a route call is not a shell call: cmd=%q isShell=%v", cmd, isShell)
	}
	if cmd, isShell := shellCall("go test ./...", "").ShellCommand(); cmd != "go test ./..." || !isShell {
		t.Fatalf("a well-formed shell call: cmd=%q isShell=%v", cmd, isShell)
	}
	// A malformed shell call is still a shell call, with no command: the shared
	// runner then allows it unchanged instead of routing it.
	empty := shellCall("", "")
	if cmd, isShell := empty.ShellCommand(); cmd != "" || !isShell {
		t.Fatalf("an empty shell call: cmd=%q isShell=%v", cmd, isShell)
	}
	badChild := shellCall("go test", "")
	badChild.Correlation.Child = "not-hex"
	if cmd, isShell := badChild.ShellCommand(); cmd != "" || !isShell {
		t.Fatalf("a shell call with a bad correlation: cmd=%q isShell=%v", cmd, isShell)
	}
}

func TestShellRewrite_AnswerReplacesOnlyTheCommand(t *testing.T) {
	out, err := shellCall("go test ./...", "").ShellRewrite("downshift compact -- go test ./...")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(out)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got["action"] != "rewrite" || got["command"] != "downshift compact -- go test ./..." || got["v"] != float64(1) {
		t.Fatalf("rewrite answer = %s", raw)
	}
	if len(got) != 3 {
		t.Fatalf("the answer carries only v, action and command: %s", raw)
	}
}

// useHome points the config lookup at a temp home for one test.
func useHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	testenv.SetHome(t, home)
	for _, name := range []string{"OPENCODE_CONFIG", "OPENCODE_CONFIG_DIR", "OPENCODE_CONFIG_CONTENT", "OPENCODE_PERMISSION", "XDG_CONFIG_HOME"} {
		t.Setenv(name, "")
	}
	t.Cleanup(opencode.SetManagedDirForTest(filepath.Join(home, "managed")))
	return home
}

func writeConfig(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestShellBlocked_PermissionRules(t *testing.T) {
	cases := []struct {
		name    string
		global  string // written to ~/.config/opencode/opencode.jsonc when non-empty
		project string // written to <dir>/.opencode/opencode.json when non-empty
		want    bool
	}{
		{name: "no config at all", want: false},
		{name: "allow-only rules do not block", global: `{"permissions":[{"action":"shell","resource":"*","effect":"allow"}]}`, want: false},
		{name: "a deny rule blocks", global: `{"permissions":[{"action":"shell","resource":"*","effect":"deny"}]}`, want: true},
		{name: "an ask rule blocks", global: `{"permission":{"bash":"ask"}}`, want: true},
		{name: "a bash allow object does not block", global: `{"permission":{"bash":"allow"}}`, want: false},
		{name: "a nested agent rule blocks", project: `{"agents":{"build":{"permission":{"bash":{"rm *":"deny"}}}}}`, want: true},
		{name: "an unreadable config blocks (fail closed)", global: `{"permissions":[`, want: true},
		{name: "JSONC comments and trailing commas parse", global: "// note\n{\n  /* none */\n  \"model\": \"x/y\",\n}\n", want: false},
		{name: "a string holding // is not a comment", global: `{"model":"https://x//y","permissions":[{"action":"shell","effect":"deny"}]}`, want: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			home := useHome(t)
			if tc.global != "" {
				writeConfig(t, filepath.Join(home, ".config", "opencode", "opencode.jsonc"), tc.global)
			}
			dir := t.TempDir()
			if tc.project != "" {
				writeConfig(t, filepath.Join(dir, ".opencode", "opencode.json"), tc.project)
			}
			if got := shellCall("go test ./...", dir).ShellBlocked("go test ./..."); got != tc.want {
				t.Fatalf("ShellBlocked = %v, want %v", got, tc.want)
			}
		})
	}
}

func retryCall(routed bool, requested opencode.RequestModel, attempt int) opencode.Request {
	return opencode.Request{
		V:           1,
		Kind:        "retry",
		Harness:     opencode.RequestHarness{ID: "opencode", Version: "2.2.5"},
		Event:       "child_retry",
		Correlation: opencode.RequestRelation{Child: shellChild},
		Retry: &opencode.RequestRetry{
			Routed:    routed,
			Requested: requested,
			ErrorType: "provider.invalid-request",
			Status:    400,
			Attempt:   attempt,
		},
	}
}

func TestHandle_RetryFallsBackOnceToTheRequestedModel(t *testing.T) {
	requested := opencode.RequestModel{ProviderID: "opencode-go", ID: "claude-haiku-5-5"}

	resp, _, decision := opencode.Handle(retryCall(true, requested, 2))
	if resp.Action != "retry" || resp.Model == nil || resp.Model.ProviderID != "opencode-go" || resp.Model.ID != "claude-haiku-5-5" {
		t.Fatalf("first failure of a routed child: %+v", resp)
	}
	if resp.ReasonCode != "routed_failed_retried" {
		t.Fatalf("reasonCode = %q", resp.ReasonCode)
	}
	if decision.Harness != "" {
		t.Fatalf("a retry is not a routing decision, so it records no route: %+v", decision)
	}

	for name, req := range map[string]opencode.Request{
		"child never routed":    retryCall(false, requested, 2),
		"third attempt":         retryCall(true, requested, 3),
		"no requested model":    retryCall(true, opencode.RequestModel{ProviderID: "opencode"}, 2),
		"retry without details": {V: 1, Kind: "retry"},
	} {
		resp, _, _ := opencode.Handle(req)
		if resp.Action != "allow" || resp.Model != nil || resp.ReasonCode != "no_retry" {
			t.Fatalf("%s: want allow with no change, got %+v", name, resp)
		}
	}
}

func TestHandle_RetryValidatesTheRequestAndTheExactAttempt(t *testing.T) {
	requested := opencode.RequestModel{ProviderID: "opencode-go", ID: "claude-haiku-5-5"}
	mutate := func(attempt int, f func(*opencode.Request)) opencode.Request {
		req := retryCall(true, requested, attempt)
		f(&req)
		return req
	}
	cases := []struct {
		name  string
		req   opencode.Request
		retry bool
	}{
		{"attempt 2 is the undo", retryCall(true, requested, 2), true},
		{"attempt 0 (absent)", retryCall(true, requested, 0), false},
		{"attempt 1", retryCall(true, requested, 1), false},
		{"attempt 3", retryCall(true, requested, 3), false},
		{"wrong wire version", mutate(2, func(r *opencode.Request) { r.V = 2 }), false},
		{"wrong harness", mutate(2, func(r *opencode.Request) { r.Harness.ID = "claude" }), false},
		{"wrong event", mutate(2, func(r *opencode.Request) { r.Event = "shell_command" }), false},
		{"child id not hex", mutate(2, func(r *opencode.Request) { r.Correlation.Child = "NOT-HEX" }), false},
		{"empty child id", mutate(2, func(r *opencode.Request) { r.Correlation.Child = "" }), false},
		{"provider id over the cap", mutate(2, func(r *opencode.Request) {
			r.Retry.Requested.ProviderID = strings.Repeat("p", 65)
		}), false},
		{"model id over the cap", mutate(2, func(r *opencode.Request) {
			r.Retry.Requested.ID = strings.Repeat("m", 129)
		}), false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			resp, _, _ := opencode.Handle(c.req)
			if c.retry {
				if resp.Action != "retry" || resp.Model == nil {
					t.Fatalf("want retry, got %+v", resp)
				}
				return
			}
			if resp.Action != "allow" || resp.Model != nil || resp.ReasonCode != "no_retry" {
				t.Fatalf("want allow no_retry, got %+v", resp)
			}
		})
	}
}
