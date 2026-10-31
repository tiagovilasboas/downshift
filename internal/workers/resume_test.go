// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package workers

import (
	"context"
	"testing"
	"time"
)

// claudeResultWithSession is the shape `claude -p --output-format json` writes
// when the session is persisted: the result object carries a session_id (field
// name confirmed in the installed CLI 2.1.316, --resume <session-id>).
const claudeResultWithSession = `{"type":"result","subtype":"success","is_error":false,"result":"done",` +
	`"session_id":"11111111-2222-3333-4444-555555555555",` +
	`"usage":{"input_tokens":2,"cache_read_input_tokens":35805,"output_tokens":9}}`

// TestSessionParsers is unhappy-path first: the three ways a session id can be
// missing must all return "" (so the job still works, just not resumable),
// then the happy path reads the real id. The claude fixture is the format the
// spike captured; the codex fixture is the shared event stream whose
// thread.started line carries the thread id.
func TestSessionParsers(t *testing.T) {
	cases := []struct {
		name  string
		parse func([]byte) string
		in    string
		want  string
	}{
		// unhappy: no session_id in the object (an older CLI that did not persist).
		{"claude no session field", claudeSession, `{"type":"result","subtype":"success","result":"x"}`, ""},
		// unhappy: the output is not JSON at all.
		{"claude malformed", claudeSession, "not json", ""},
		// unhappy: an empty session_id is the same as none.
		{"claude empty session", claudeSession, `{"session_id":""}`, ""},
		// unhappy: no thread.started line in the codex event stream.
		{"codex no thread line", codexSession, `{"type":"turn.completed"}` + "\n", ""},
		// unhappy: codex wrote nothing.
		{"codex empty", codexSession, "", ""},
		// happy: the real id is read.
		{"claude reads the id", claudeSession, claudeResultWithSession, "11111111-2222-3333-4444-555555555555"},
		{"codex reads the thread id", codexSession, codexEvents, "t1"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.parse([]byte(c.in)); got != c.want {
				t.Fatalf("got %q, want %q", got, c.want)
			}
		})
	}
}

// TestResumeArgsStayInsideTheLauncher checks each launcher builds the resume
// args of its own CLI, with the id in a position a malformed id cannot turn
// into a flag, and the isolation unchanged. The queue never sees these args:
// they live next to the launcher (owner design).
func TestResumeArgsStayInsideTheLauncher(t *testing.T) {
	id := "11111111-2222-3333-4444-555555555555"
	claude := claudeResumeArgs(id)
	if !containsPair(claude, "--resume", id) {
		t.Errorf("claude resume args miss --resume <id>: %v", claude)
	}
	if !contains(claude, "--strict-mcp-config") {
		t.Errorf("claude resume must keep the isolation: %v", claude)
	}
	if claude[len(claude)-1] != id {
		t.Errorf("the id must be the last arg so it cannot shift a flag: %v", claude)
	}
	codex := codexResumeArgs(id)
	// codex exec resume <id> ... : the id is the positional argument after resume.
	if len(codex) < 3 || codex[0] != "exec" || codex[1] != "resume" || codex[2] != id {
		t.Errorf("codex resume args must be exec resume <id> ...: %v", codex)
	}
	if !contains(codex, "read-only") || !contains(codex, "--json") {
		t.Errorf("codex resume must keep the read-only sandbox and JSON events: %v", codex)
	}
}

// TestClaudeWorkerCapturesItsSessionIdIntoCollectAndStatus runs a claude shim
// that writes the real result shape and checks the id reaches both the
// delivery (collect) and the status view.
func TestClaudeWorkerCapturesItsSessionIdIntoCollectAndStatus(t *testing.T) {
	shim(t, "claude", `echo '`+claudeResultWithSession+`'`)
	q := New(ByAgent(ReadOnlyLaunchers()), 1, fixedNow)
	if _, err := q.Delegate([]Spec{{Agent: "claude", Dir: t.TempDir(), Task: "go", Budget: time.Minute}}); err != nil {
		t.Fatal(err)
	}
	c := q.Collect(context.Background(), 0, true, 10*time.Second)
	if len(c.Deliveries) != 1 {
		t.Fatalf("deliveries = %+v", c.Deliveries)
	}
	const want = "11111111-2222-3333-4444-555555555555"
	if c.Deliveries[0].SessionID != want {
		t.Errorf("delivery session id = %q, want %q", c.Deliveries[0].SessionID, want)
	}
	views := q.Workers()
	if len(views) != 1 || views[0].SessionID != want {
		t.Errorf("status session id = %+v, want %q", views, want)
	}
}

// TestCodexWorkerCapturesItsThreadIdIntoCollect runs a codex shim that writes
// the real event stream (whose thread.started line carries the thread id) and
// checks the id reaches the delivery.
func TestCodexWorkerCapturesItsThreadIdIntoCollect(t *testing.T) {
	body := `out=""; while [ $# -gt 0 ]; do if [ "$1" = "-o" ]; then out="$2"; fi; shift; done; printf 'final' > "$out"; cat <<'EOF'
` + codexEvents + `EOF`
	shim(t, "codex", body)
	q := New(ByAgent(ReadOnlyLaunchers()), 1, fixedNow)
	if _, err := q.Delegate([]Spec{{Agent: "codex", Dir: t.TempDir(), Task: "go", Budget: time.Minute}}); err != nil {
		t.Fatal(err)
	}
	c := q.Collect(context.Background(), 0, true, 10*time.Second)
	if len(c.Deliveries) != 1 || c.Deliveries[0].SessionID != "t1" {
		t.Fatalf("delivery = %+v, want session id t1", c.Deliveries)
	}
}

// TestASessionlessWorkerStillDelivers is the unhappy end-to-end: an older CLI
// whose output carries no session id. The job must still deliver, with an empty
// session id and nothing broken.
func TestASessionlessWorkerStillDelivers(t *testing.T) {
	shim(t, "claude", `echo '{"type":"result","subtype":"success","is_error":false,"result":"done"}'`)
	q := New(ByAgent(ReadOnlyLaunchers()), 1, fixedNow)
	if _, err := q.Delegate([]Spec{{Agent: "claude", Dir: t.TempDir(), Task: "go", Budget: time.Minute}}); err != nil {
		t.Fatal(err)
	}
	c := q.Collect(context.Background(), 0, true, 10*time.Second)
	if len(c.Deliveries) != 1 || c.Deliveries[0].Outcome != "done" {
		t.Fatalf("deliveries = %+v", c.Deliveries)
	}
	if c.Deliveries[0].SessionID != "" {
		t.Errorf("a worker that wrote no session id must deliver an empty one, got %q", c.Deliveries[0].SessionID)
	}
}

// TestMalformedOutputDoesNotCaptureASession is the other unhappy end-to-end:
// the worker's output is not the JSON the parser expects, so the job fails and
// no session id is captured.
func TestMalformedOutputDoesNotCaptureASession(t *testing.T) {
	shim(t, "claude", `echo 'garbage not json'`)
	q := New(ByAgent(ReadOnlyLaunchers()), 1, fixedNow)
	if _, err := q.Delegate([]Spec{{Agent: "claude", Dir: t.TempDir(), Task: "go", Budget: time.Minute}}); err != nil {
		t.Fatal(err)
	}
	c := q.Collect(context.Background(), 0, true, 10*time.Second)
	if len(c.Deliveries) != 1 || c.Deliveries[0].Outcome != "failed" {
		t.Fatalf("deliveries = %+v, want a failed delivery", c.Deliveries)
	}
	if c.Deliveries[0].SessionID != "" {
		t.Errorf("malformed output must capture no session id, got %q", c.Deliveries[0].SessionID)
	}
}

func contains(s []string, want string) bool {
	for _, v := range s {
		if v == want {
			return true
		}
	}
	return false
}

func containsPair(s []string, a, b string) bool {
	for i := 0; i+1 < len(s); i++ {
		if s[i] == a && s[i+1] == b {
			return true
		}
	}
	return false
}
