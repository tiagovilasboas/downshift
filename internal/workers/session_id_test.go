// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package workers

import (
	"context"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// The Downshift knows a worker's session id before the process starts, whenever
// the CLI lets it: the Claude launcher generates a UUID and passes --session-id,
// so a cancel on the very first turn (a SIGKILL before the CLI writes its JSON)
// still has an id to delete the transcript with. These tests are unhappy-path
// first (regra 1:3): the Codex "no change" and the first-turn cancel come before
// the happy args and resume checks. They drive the real launchers through the
// launch shim and point HOME at a fake dir so no real transcript is touched.

// uuidV4 is the shape the Claude launcher must generate: 8-4-4-4-12 hex with the
// version nibble 4 and the variant nibble in 8-b.
var uuidV4 = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

// claudeSessionIDArg returns the --session-id value the shim recorded, or "".
func claudeSessionIDArg(args string) string {
	lines := strings.Split(strings.TrimRight(args, "\n"), "\n")
	for i, a := range lines {
		if a == "--session-id" && i+1 < len(lines) {
			return lines[i+1]
		}
	}
	return ""
}

// --- unhappy: Codex gets no --session-id (it has no upfront-id flag) ---

// TestCodexStillHasNoSessionIdFlag proves the Codex launch is unchanged: codex
// exec has no way to set the thread id before the run, so the recipe must not
// grow a --session-id and must keep its read-only sandbox.
func TestCodexStillHasNoSessionIdFlag(t *testing.T) {
	dir := shim(t, "codex", `out=""; while [ $# -gt 0 ]; do if [ "$1" = "-o" ]; then out="$2"; fi; shift; done; printf 'ok' > "$out"`)
	proc, err := ReadOnlyLaunchers()["codex"].Launch(Spec{Agent: "codex", Dir: t.TempDir(), Task: "x", Budget: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	if err := proc.Wait(); err != nil {
		t.Fatal(err)
	}
	args := readFile(t, filepath.Join(dir, "codex.args"))
	if strings.Contains(args, "--session-id") {
		t.Errorf("codex must not grow a --session-id: %s", args)
	}
	if !strings.Contains(args, "read-only") {
		t.Errorf("codex must keep its read-only sandbox: %s", args)
	}
	// A codex worker reports no preset id before it runs: nothing to know upfront.
	if pr, ok := proc.(presetSessionReporter); ok && pr.PresetSessionID() != "" {
		t.Errorf("codex must report no preset session id, got %q", pr.PresetSessionID())
	}
}

// --- unhappy: a cancel on the first turn still deletes the transcript ---

// TestCancelOnTheFirstTurnDeletesTheClaudeTranscript is the gap the change
// closes. A worker is killed with SIGKILL, which the CLI cannot trap, so on the
// first turn the CLI never writes its final JSON and the queue would capture no
// id from the output. Because the launcher now knows the id upfront, the queue
// has an id the moment the worker starts, so a first-turn cancel deletes the
// transcript the shim wrote. The shim blocks until killed and, like the real
// CLI, writes its transcript to ~/.claude/projects/<dir>/<id>.jsonl using the
// id it was handed on --session-id, then is killed before it can print JSON.
func TestCancelOnTheFirstTurnDeletesTheClaudeTranscript(t *testing.T) {
	home := fakeHome(t)
	work := t.TempDir()
	projects := filepath.Join(home, ".claude", "projects", claudeProjectFolder(work))
	// The shim mimics the real CLI: it reads --session-id, writes the transcript
	// keyed by the working directory with that id, then blocks forever (so the
	// cancel's SIGKILL is what ends it, before any JSON is printed to stdout).
	body := `sid=""; while [ $# -gt 0 ]; do if [ "$1" = "--session-id" ]; then sid="$2"; fi; shift; done
mkdir -p ` + projects + `
printf '{}\n' > ` + projects + `/"$sid".jsonl
while true; do sleep 1; done`
	shim(t, "claude", body)

	q := New(ByAgent(ReadOnlyLaunchers()), 1, fixedNow)
	if _, err := q.Delegate([]Spec{{Agent: "claude", Dir: work, Task: "go", Budget: time.Minute}}); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the worker to run", func() bool { return q.Statuses()[0].State == Running })

	// The id is known upfront, so it is on the job before the worker writes JSON.
	q.mu.Lock()
	id := q.jobs["w001"].sessionID
	q.mu.Unlock()
	if !uuidV4.MatchString(id) {
		t.Fatalf("the queue must know the session id upfront, got %q", id)
	}
	jsonl := filepath.Join(projects, id+".jsonl")
	waitFor(t, "the shim to write its transcript", func() bool { return exists(jsonl) })

	q.Cancel([]string{"w001"})
	waitFor(t, "the first-turn transcript to be deleted", func() bool { return !exists(jsonl) })
}

// --- happy: the Claude launch passes a valid UUID on --session-id ---

// TestClaudeLaunchPassesAValidUUIDUpfront proves the read-only launch generates
// a UUID v4, passes it as --session-id, keeps the isolation, and reports the
// same id as its preset before the process even exits.
func TestClaudeLaunchPassesAValidUUIDUpfront(t *testing.T) {
	dir := shim(t, "claude", `echo '{"type":"result","subtype":"success","is_error":false,"result":"ok"}'`)
	proc, err := ReadOnlyLaunchers()["claude"].Launch(Spec{Agent: "claude", Dir: t.TempDir(), Task: "x", Budget: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	pr, ok := proc.(presetSessionReporter)
	if !ok {
		t.Fatal("the claude process must report a preset session id")
	}
	preset := pr.PresetSessionID()
	if !uuidV4.MatchString(preset) {
		t.Errorf("preset session id is not a UUID v4: %q", preset)
	}
	if err := proc.Wait(); err != nil {
		t.Fatal(err)
	}
	args := readFile(t, filepath.Join(dir, "claude.args"))
	if got := claudeSessionIDArg(args); got != preset {
		t.Errorf("--session-id arg = %q, want the preset %q (args: %s)", got, preset, args)
	}
	if !strings.Contains(args, "--strict-mcp-config") {
		t.Errorf("the isolation must stay with the upfront id: %s", args)
	}
	// The preset is also what the worker reports as its session once it exits, so
	// the capture after the run agrees with the id known upfront.
	if sr, ok := proc.(sessionReporter); ok && sr.SessionID() != preset {
		t.Errorf("SessionID after exit = %q, want the preset %q", sr.SessionID(), preset)
	}
}

// TestEachClaudeWorkerGetsAUniqueSessionId proves two workers started from the
// same recipe get different ids, so one worker's transcript can never collide
// with another's.
func TestEachClaudeWorkerGetsAUniqueSessionId(t *testing.T) {
	shim(t, "claude", `echo '{"type":"result","subtype":"success","is_error":false,"result":"ok"}'`)
	seen := map[string]bool{}
	for i := 0; i < 5; i++ {
		proc, err := ReadOnlyLaunchers()["claude"].Launch(Spec{Agent: "claude", Dir: t.TempDir(), Task: "x", Budget: time.Minute})
		if err != nil {
			t.Fatal(err)
		}
		id := proc.(presetSessionReporter).PresetSessionID()
		if !uuidV4.MatchString(id) {
			t.Fatalf("id %q is not a UUID v4", id)
		}
		if seen[id] {
			t.Fatalf("session id %q was reused across workers", id)
		}
		seen[id] = true
		_ = proc.Wait()
	}
}

// --- happy: a resumed turn reuses the existing id, never a new one ---

// TestResumeReusesTheExistingSessionId proves a resumed turn continues the same
// conversation: the launcher resumes with --resume <id> and passes no new
// --session-id, so the id a first turn established is the one every later turn
// uses.
func TestResumeReusesTheExistingSessionId(t *testing.T) {
	const existing = "11111111-2222-3333-4444-555555555555"
	dir := shim(t, "claude", `echo '{"type":"result","subtype":"success","is_error":false,"result":"ok","session_id":"`+existing+`"}'`)
	proc, err := ReadOnlyLaunchers()["claude"].(cliLauncher).LaunchResume(
		Spec{Agent: "claude", Dir: t.TempDir(), Task: "next", Budget: time.Minute}, existing)
	if err != nil {
		t.Fatal(err)
	}
	if err := proc.Wait(); err != nil {
		t.Fatal(err)
	}
	args := readFile(t, filepath.Join(dir, "claude.args"))
	if !strings.Contains(args, "--resume\n"+existing) && !strings.Contains(args, existing) {
		t.Errorf("resume must continue the existing id: %s", args)
	}
	if strings.Contains(args, "--session-id") {
		t.Errorf("a resume must not open a new session with --session-id: %s", args)
	}
	// The preset on a resumed process is the existing id, not a freshly generated
	// one: the resumed turn reuses the conversation.
	if pr, ok := proc.(presetSessionReporter); ok && pr.PresetSessionID() != existing {
		t.Errorf("resumed preset = %q, want the existing id %q", pr.PresetSessionID(), existing)
	}
}

// TestDelegateRecordsTheUpfrontIdThroughCollect is the end-to-end happy path:
// a delegated claude worker's id is on the job from the start and the same id
// comes back on the delivery.
func TestDelegateRecordsTheUpfrontIdThroughCollect(t *testing.T) {
	fakeHome(t)
	shim(t, "claude", `sid=""; while [ $# -gt 0 ]; do if [ "$1" = "--session-id" ]; then sid="$2"; fi; shift; done
echo '{"type":"result","subtype":"success","is_error":false,"result":"ok","session_id":"'"$sid"'"}'`)
	q := New(ByAgent(ReadOnlyLaunchers()), 1, fixedNow)
	if _, err := q.Delegate([]Spec{{Agent: "claude", Dir: t.TempDir(), Task: "go", Budget: time.Minute}}); err != nil {
		t.Fatal(err)
	}
	c := q.Collect(context.Background(), 0, true, 10*time.Second)
	if len(c.Deliveries) != 1 {
		t.Fatalf("deliveries = %+v", c.Deliveries)
	}
	if !uuidV4.MatchString(c.Deliveries[0].SessionID) {
		t.Errorf("delivery session id is not the upfront UUID: %q", c.Deliveries[0].SessionID)
	}
}
