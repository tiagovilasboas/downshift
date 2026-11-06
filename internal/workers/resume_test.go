// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package workers

import (
	"context"
	"strings"
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
	claude := claudeResumeArgs("plan", Spec{}, id)
	if !containsPair(claude, "--resume", id) {
		t.Errorf("claude resume args miss --resume <id>: %v", claude)
	}
	if !contains(claude, "--strict-mcp-config") {
		t.Errorf("claude resume must keep the isolation: %v", claude)
	}
	if claude[len(claude)-1] != id {
		t.Errorf("the id must be the last arg so it cannot shift a flag: %v", claude)
	}
	codex := codexResumeArgs("read-only", false, Spec{}, "/tmp/report.txt", id)
	// codex exec resume <id> ... : the id is the positional argument after resume.
	if len(codex) < 3 || codex[0] != "exec" || codex[1] != "resume" || codex[2] != id {
		t.Errorf("codex resume args must be exec resume <id> ...: %v", codex)
	}
	if !contains(codex, "sandbox_mode=\"read-only\"") || !contains(codex, "--json") {
		t.Errorf("codex resume must keep the read-only sandbox and JSON events: %v", codex)
	}
	if !containsPair(codex, "-o", "/tmp/report.txt") {
		t.Errorf("codex resume must name the report file with -o: %v", codex)
	}
}

// TestACodexResumedTurnDeliversItsFinalMessage resumes a codex worker through
// the real launcher with a shim that writes its final message only where -o
// points: the resumed turn's report is that message, not the event stream.
func TestACodexResumedTurnDeliversItsFinalMessage(t *testing.T) {
	shim(t, "codex", `out=""; while [ $# -gt 0 ]; do if [ "$1" = "-o" ]; then out="$2"; fi; shift; done; [ -n "$out" ] || exit 3; printf 'resumed answer' > "$out"; echo '{"type":"turn.completed"}'`)
	proc, err := cliLauncher{plan: codexReadOnly}.LaunchResume(Spec{Agent: "codex", Dir: t.TempDir(), Task: "again", Budget: time.Minute}, "thread-1")
	if err != nil {
		t.Fatal(err)
	}
	if err := proc.Wait(); err != nil {
		t.Fatalf("the resumed turn failed (no -o?): %v", err)
	}
	if got := proc.Report(); got != "resumed answer" {
		t.Errorf("resumed report = %q, want the final message", got)
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

// TestALongCodexRunKeepsItsThreadId runs a codex shim whose event stream is far
// longer than the output tail: the thread.started line comes first and would
// roll out of the tail, leaving the worker neither resumable nor cleanable.
func TestALongCodexRunKeepsItsThreadId(t *testing.T) {
	body := `out=""; while [ $# -gt 0 ]; do if [ "$1" = "-o" ]; then out="$2"; fi; shift; done; printf 'final' > "$out"
echo '{"type":"thread.started","thread_id":"long-1"}'
i=0; while [ $i -lt 2000 ]; do echo '{"type":"item.completed","item":{"id":"x","type":"agent_message","text":"padding padding padding padding padding"}}'; i=$((i+1)); done`
	shim(t, "codex", body)
	q := New(ByAgent(ReadOnlyLaunchers()), 1, fixedNow)
	if _, err := q.Delegate([]Spec{{Agent: "codex", Dir: t.TempDir(), Task: "go", Budget: time.Minute}}); err != nil {
		t.Fatal(err)
	}
	c := q.Collect(context.Background(), 0, true, 20*time.Second)
	if len(c.Deliveries) != 1 || c.Deliveries[0].SessionID != "long-1" {
		t.Fatalf("delivery = %+v, want session id long-1 from the head of the stream", c.Deliveries)
	}
}

// TestASessionlessClaudeOutputStillDeliversWithTheUpfrontId is the unhappy
// end-to-end for a claude worker whose output carries no session_id (an older
// CLI, say). The job must still deliver; and because the launcher now sets the
// id with --session-id before the run, the delivery still carries that upfront
// id, so the worker is resumable and cleanable even when its output said
// nothing. (Before the upfront id, this delivered an empty id; the id is now
// always known for Claude.)
func TestASessionlessClaudeOutputStillDeliversWithTheUpfrontId(t *testing.T) {
	shim(t, "claude", `echo '{"type":"result","subtype":"success","is_error":false,"result":"done"}'`)
	q := New(ByAgent(ReadOnlyLaunchers()), 1, fixedNow)
	if _, err := q.Delegate([]Spec{{Agent: "claude", Dir: t.TempDir(), Task: "go", Budget: time.Minute}}); err != nil {
		t.Fatal(err)
	}
	c := q.Collect(context.Background(), 0, true, 10*time.Second)
	if len(c.Deliveries) != 1 || c.Deliveries[0].Outcome != "done" {
		t.Fatalf("deliveries = %+v", c.Deliveries)
	}
	if !uuidV4.MatchString(c.Deliveries[0].SessionID) {
		t.Errorf("the delivery must carry the upfront id even when the output had none, got %q", c.Deliveries[0].SessionID)
	}
}

// TestMalformedClaudeOutputStillFailsButKeepsTheUpfrontId is the other unhappy
// end-to-end: the worker's output is not the JSON the parser expects, so the
// job fails. The session id is still the one set upfront, so a failed first
// turn leaves no un-cleanable transcript behind.
func TestMalformedClaudeOutputStillFailsButKeepsTheUpfrontId(t *testing.T) {
	shim(t, "claude", `echo 'garbage not json'`)
	q := New(ByAgent(ReadOnlyLaunchers()), 1, fixedNow)
	if _, err := q.Delegate([]Spec{{Agent: "claude", Dir: t.TempDir(), Task: "go", Budget: time.Minute}}); err != nil {
		t.Fatal(err)
	}
	c := q.Collect(context.Background(), 0, true, 10*time.Second)
	if len(c.Deliveries) != 1 || c.Deliveries[0].Outcome != "failed" {
		t.Fatalf("deliveries = %+v, want a failed delivery", c.Deliveries)
	}
	if !uuidV4.MatchString(c.Deliveries[0].SessionID) {
		t.Errorf("a failed claude turn must keep the upfront id for cleanup, got %q", c.Deliveries[0].SessionID)
	}
}

// TestACodexWorkerWithNoThreadLineDeliversAnEmptyId keeps the "sessionless"
// case for the CLI that has no upfront-id flag: Codex reports its thread id only
// at runtime, so a Codex run whose event stream carries no thread.started line
// delivers an empty id (not resumable), and nothing breaks.
func TestACodexWorkerWithNoThreadLineDeliversAnEmptyId(t *testing.T) {
	body := `out=""; while [ $# -gt 0 ]; do if [ "$1" = "-o" ]; then out="$2"; fi; shift; done; printf 'final' > "$out"; echo '{"type":"turn.completed"}'`
	shim(t, "codex", body)
	q := New(ByAgent(ReadOnlyLaunchers()), 1, fixedNow)
	if _, err := q.Delegate([]Spec{{Agent: "codex", Dir: t.TempDir(), Task: "go", Budget: time.Minute}}); err != nil {
		t.Fatal(err)
	}
	c := q.Collect(context.Background(), 0, true, 10*time.Second)
	if len(c.Deliveries) != 1 || c.Deliveries[0].Outcome != "done" {
		t.Fatalf("deliveries = %+v", c.Deliveries)
	}
	if c.Deliveries[0].SessionID != "" {
		t.Errorf("a codex worker with no thread line must deliver an empty id, got %q", c.Deliveries[0].SessionID)
	}
}

// TestRoutesForwardsResumeSoASettledWorkerResumes is the regression for the
// send-resume defect: the real MCP server builds its queue through Routes
// (cmd/downshift workerLaunchers), and Routes must forward LaunchResume to the
// launcher of the worker's mode, exactly as it forwards CleanupSession. Before
// the fix, routes implemented Launch and CleanupSession but not LaunchResume,
// so canResume's launcher check was always false and a settled Claude worker
// that captured a real session id was still refused a second turn
// (ErrNotResumable), even though its inner launcher could resume it.
//
// Unhappy-path first (regra 1:3), all through the same Routes launcher:
//   - a settled worker that captured no id is refused;
//   - a cancelled worker is refused;
//   - then the happy path: a settled done worker with an id resumes.
func TestRoutesForwardsResumeSoASettledWorkerResumes(t *testing.T) {
	const sid = "11111111-2222-3333-4444-555555555555"

	// read is the live read route the server uses; write is nil here (read-only
	// server), so Routes must still forward resume to the read launcher.
	routed := func() Launcher { return Routes(ByAgent(ReadOnlyLaunchers()), nil) }

	// unhappy: a settled Codex worker whose event stream carries no thread id has
	// no session to resume, so a second turn is refused even through Routes.
	t.Run("settled without a session id is refused", func(t *testing.T) {
		body := `out=""; while [ $# -gt 0 ]; do if [ "$1" = "-o" ]; then out="$2"; fi; shift; done; printf 'final' > "$out"; echo '{"type":"turn.completed"}'`
		shim(t, "codex", body)
		q := New(routed(), 1, fixedNow)
		if _, err := q.Delegate([]Spec{{Agent: "codex", Dir: t.TempDir(), Task: "t1", Budget: time.Minute}}); err != nil {
			t.Fatal(err)
		}
		c := q.Collect(context.Background(), 0, true, 10*time.Second)
		if len(c.Deliveries) != 1 || c.Deliveries[0].SessionID != "" {
			t.Fatalf("precondition: want one delivery with empty id, got %+v", c.Deliveries)
		}
		if err := q.Continue("w001", "again"); err != ErrNotResumable {
			t.Fatalf("Continue on a sessionless worker = %v, want ErrNotResumable", err)
		}
	})

	// unhappy: a cancelled worker is refused even if it captured an upfront id.
	t.Run("cancelled is refused", func(t *testing.T) {
		shim(t, "claude", `sleep 5; echo '{"type":"result","subtype":"success","result":"ok","session_id":"`+sid+`"}'`)
		q := New(routed(), 1, fixedNow)
		if _, err := q.Delegate([]Spec{{Agent: "claude", Dir: t.TempDir(), Task: "t1", Budget: time.Minute}}); err != nil {
			t.Fatal(err)
		}
		waitFor(t, "worker to start", func() bool { return q.Statuses()[0].State == Running })
		q.Cancel([]string{"w001"})
		c := q.Collect(context.Background(), 0, true, 10*time.Second)
		if len(c.Deliveries) != 1 || c.Deliveries[0].Outcome != "cancelled" {
			t.Fatalf("precondition: want one cancelled delivery, got %+v", c.Deliveries)
		}
		if err := q.Continue("w001", "again"); err != ErrNotResumable {
			t.Fatalf("Continue on a cancelled worker = %v, want ErrNotResumable", err)
		}
	})

	// happy: a settled Claude worker that ended done with a captured id resumes,
	// through Routes, running --resume <that id> in the same dir. This is the
	// path the live proof exercised and the one the defect broke.
	t.Run("settled done with an id resumes", func(t *testing.T) {
		dir := shim(t, "claude", `echo '{"type":"result","subtype":"success","is_error":false,"result":"ok","session_id":"`+sid+`"}'`)
		work := t.TempDir()
		q := New(routed(), 1, fixedNow)
		if _, err := q.Delegate([]Spec{{Agent: "claude", Dir: work, Task: "turn-1", Budget: time.Minute}}); err != nil {
			t.Fatal(err)
		}
		c1 := q.Collect(context.Background(), 0, true, 10*time.Second)
		if len(c1.Deliveries) != 1 || c1.Deliveries[0].SessionID != sid {
			t.Fatalf("first delivery = %+v, want session %s", c1.Deliveries, sid)
		}
		if err := q.Continue("w001", "turn-2"); err != nil {
			t.Fatalf("resume through Routes was refused: %v", err)
		}
		c2 := q.Collect(context.Background(), c1.Ack, true, 10*time.Second)
		if len(c2.Deliveries) != 1 || c2.Deliveries[0].Job != "w001" {
			t.Fatalf("second delivery = %+v, want one on w001", c2.Deliveries)
		}
		args := readFile(t, dir+"/claude.args")
		if !strings.Contains(args, "--resume") || !strings.Contains(args, sid) {
			t.Errorf("the resumed turn did not run --resume <id>: %q", args)
		}
	})
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
