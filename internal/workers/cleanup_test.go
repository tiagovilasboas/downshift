// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package workers

import (
	"context"
	"errors"
	"github.com/tiagovilasboas/downshift/internal/testenv"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// A worker's session is persisted so a later turn can resume it; this file's
// counterpart deletes that transcript when the queue is done with the worker
// (release, cancel, server close). These tests drive the real launchers through
// the shim CLI so a session id is captured, and point HOME at a temporary
// directory so every deletion happens in a fake config dir, never the real one.
// They are unhappy-path first (regra 1:3): the safety refusals come before the
// happy deletions.

const cleanSessionID = "11111111-2222-3333-4444-555555555555"

// fakeHome points HOME (and the Windows-ish fallbacks) at a fresh temp dir, so
// userConfigRoot resolves inside it and no real session file is ever touched.
func fakeHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	testenv.SetHome(t, home)
	return home
}

// claudeShimWith starts a claude shim that reports the given session id, so a
// delegated worker finishes resumable.
func claudeShimWith(t *testing.T, id string) {
	t.Helper()
	shim(t, "claude", `echo '{"type":"result","subtype":"success","is_error":false,"result":"ok","session_id":"`+id+`"}'`)
}

// writeClaudeTranscript creates the transcript file Claude would have written
// for a worker that ran in dir with the given session id, plus the session-env
// directory the CLI keys by the id, both under the fake home. It returns the
// two paths so a test can assert they are gone (or still there).
func writeClaudeTranscript(t *testing.T, home, dir, id string) (jsonl, envDir string) {
	t.Helper()
	projects := filepath.Join(home, ".claude", "projects", claudeProjectFolder(dir))
	if err := os.MkdirAll(projects, 0o755); err != nil {
		t.Fatal(err)
	}
	jsonl = filepath.Join(projects, id+".jsonl")
	if err := os.WriteFile(jsonl, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	envDir = filepath.Join(home, ".claude", "session-env", id)
	if err := os.MkdirAll(envDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(envDir, "env"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	return jsonl, envDir
}

func exists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

// --- unhappy: the deletion refuses an unsafe id and never leaves the root ---

// TestCleanRefusesAnIdWithParentReference proves a session id carrying ".." or
// a separator deletes nothing: a decoy file outside the config root survives,
// and the valid-looking transcript is left untouched (the id never names it).
func TestCleanRefusesAnUnsafeId(t *testing.T) {
	home := fakeHome(t)
	// A decoy the id would reach if ".." were allowed to climb out of projects.
	outside := filepath.Join(home, "SECRET.jsonl")
	if err := os.WriteFile(outside, []byte("keep me"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{
		"../../SECRET",  // climbs out of the projects folder
		"a/b",           // a separator
		"..",            // the parent itself
		"*",             // a glob
		"",              // empty
		"id with space", // not a plain token
	} {
		if validSessionID(bad) {
			t.Errorf("validSessionID(%q) = true, want false", bad)
		}
		// Even called directly, an unsafe id deletes nothing.
		if err := claudeCleanSession(home, bad); err != nil {
			t.Errorf("claudeCleanSession(%q) = %v, want nil (no-op)", bad, err)
		}
	}
	if !exists(outside) {
		t.Fatal("an unsafe id deleted a file outside the config root")
	}
}

// TestCleanDoesNotFollowASymlink proves the delete never follows a symlink: a
// transcript path that is a symlink to a file outside the root has its link
// removed, but the real target survives, so a planted link cannot redirect the
// delete onto another file.
func TestCleanDoesNotFollowASymlink(t *testing.T) {
	home := fakeHome(t)
	work := t.TempDir()
	// The real file a malicious link would point at, outside the config root.
	target := filepath.Join(home, "real-outside.txt")
	if err := os.WriteFile(target, []byte("precious"), 0o644); err != nil {
		t.Fatal(err)
	}
	projects := filepath.Join(home, ".claude", "projects", claudeProjectFolder(work))
	if err := os.MkdirAll(projects, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(projects, cleanSessionID+".jsonl")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := claudeCleanSession(work, cleanSessionID); err != nil {
		t.Fatalf("clean: %v", err)
	}
	if exists(link) {
		t.Error("the symlink itself should be removed")
	}
	if !exists(target) {
		t.Fatal("the delete followed a symlink and removed the real target outside the root")
	}
}

// TestCleanTreatsAMissingFileAsSuccess proves a session with no file on disk is
// not an error: release and cancel must not fail because the transcript is
// already gone.
func TestCleanTreatsAMissingFileAsSuccess(t *testing.T) {
	home := fakeHome(t)
	if err := claudeCleanSession(home, cleanSessionID); err != nil {
		t.Fatalf("a missing transcript must be success, got %v", err)
	}
}

// --- unhappy: Release refuses the ids it must not drop ---

func TestReleaseRefusesAnUnknownId(t *testing.T) {
	q := New(newFakeLauncher(), 1, fixedNow)
	released, _, refused := q.Release([]string{"w404"})
	if len(released) != 0 {
		t.Errorf("released = %v, want none", released)
	}
	if refused["w404"] == "" {
		t.Errorf("an unknown id must be refused with a reason, got %v", refused)
	}
}

func TestReleaseRefusesALiveWorker(t *testing.T) {
	l := newFakeLauncher()
	q := New(l, 1, fixedNow)
	ids, err := q.Delegate([]Spec{spec("run", time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the worker to run", func() bool { return q.Statuses()[0].State == Running })
	released, _, refused := q.Release(ids)
	if len(released) != 0 {
		t.Errorf("a running worker must not be released, got %v", released)
	}
	if refused[ids[0]] == "" {
		t.Errorf("a live worker must be refused with a reason, got %v", refused)
	}
	// The worker is still there: refusing to release leaves it alive.
	if len(q.Statuses()) != 1 {
		t.Errorf("the refused worker was dropped: %+v", q.Statuses())
	}
}

// --- happy: release deletes the transcript and drops the job ---

// TestReleaseDeletesTheClaudeTranscriptAndSessionEnv runs a claude worker to a
// resumable finish, then releases it: the transcript and the session-env dir
// are gone, and the job no longer appears in the queue.
func TestReleaseDeletesTheClaudeTranscriptAndSessionEnv(t *testing.T) {
	home := fakeHome(t)
	claudeShimWith(t, cleanSessionID)
	work := t.TempDir()
	q := New(ByAgent(ReadOnlyLaunchers()), 1, fixedNow)
	if _, err := q.Delegate([]Spec{{Agent: "claude", Dir: work, Task: "go", Budget: time.Minute}}); err != nil {
		t.Fatal(err)
	}
	c := q.Collect(context.Background(), 0, true, 10*time.Second)
	if len(c.Deliveries) != 1 || c.Deliveries[0].SessionID != cleanSessionID {
		t.Fatalf("delivery = %+v, want the session id captured", c.Deliveries)
	}
	jsonl, envDir := writeClaudeTranscript(t, home, work, cleanSessionID)

	released, warnings, refused := q.Release([]string{"w001"})
	if len(released) != 1 || released[0] != "w001" {
		t.Fatalf("released = %v, want w001 (warnings %v, refused %v)", released, warnings, refused)
	}
	if exists(jsonl) {
		t.Errorf("the transcript was not deleted: %s", jsonl)
	}
	if exists(envDir) {
		t.Errorf("the session-env dir was not deleted: %s", envDir)
	}
	if len(q.Statuses()) != 0 {
		t.Errorf("the released job still appears: %+v", q.Statuses())
	}
}

// cleanRecorder is a resumable launcher that also records every session it was
// asked to delete, so a test can prove the queue calls the cleanup on the right
// path with the right (dir, id) without any CLI or disk. It reports a fixed
// session id so a settled worker is resumable and has an id to clean up.
type cleanRecorder struct {
	*resumeLaunch
	cleaned []resumeCall // each CleanupSession as (spec, session id)
}

func newCleanRecorder() *cleanRecorder { return &cleanRecorder{resumeLaunch: newResumeLaunch()} }

func (l *cleanRecorder) CleanupSession(spec Spec, sessionID string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.cleaned = append(l.cleaned, resumeCall{spec: spec, session: sessionID})
	return nil
}

func (l *cleanRecorder) cleanups() []resumeCall {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]resumeCall(nil), l.cleaned...)
}

// TestCancelCleansUpAResumedWorkersSession proves the queue deletes the session
// of a worker cancel kills, once that worker has a captured id. A worker is
// killed with SIGKILL, which it cannot trap, so a worker cancelled on its very
// first turn reports no session id (nothing to clean up yet). This drives the
// real path with a controllable launcher: a first turn settles and captures the
// id, the worker is resumed and runs again, and is cancelled mid-resume; the id
// from the first turn is on the job, so cancel asks the launcher to clean it up.
func TestCancelCleansUpAResumedWorkersSession(t *testing.T) {
	l := newCleanRecorder()
	q := New(l, 1, fixedNow)
	if _, err := q.Delegate([]Spec{continueSpec("turn-1", time.Minute)}); err != nil {
		t.Fatal(err)
	}
	l.proc("turn-1").finish("first", nil) // captures the session id (p.session = task)
	q.Collect(context.Background(), 0, true, 5*time.Second)

	if err := q.Continue("w001", "turn-2"); err != nil { // resumes, runs again
		t.Fatalf("resume: %v", err)
	}
	waitFor(t, "the resumed turn to run", func() bool { return q.Statuses()[0].State == Running })
	q.Cancel([]string{"w001"})

	waitFor(t, "the cancelled worker's session to be cleaned", func() bool {
		return len(l.cleanups()) == 1
	})
	if got := l.cleanups()[0].session; got != "turn-1" {
		t.Errorf("cleaned session = %q, want turn-1 (the id captured on the first turn)", got)
	}
}

// TestCloseReleasesASettledWorkersSession proves the server exit deletes the
// transcript of a worker that finished and was never released: nothing can
// resume it once the server is gone, so Close releases it like release would.
func TestCloseReleasesASettledWorkersSession(t *testing.T) {
	l := newCleanRecorder()
	q := New(l, 1, fixedNow)
	if _, err := q.Delegate([]Spec{continueSpec("turn-1", time.Minute)}); err != nil {
		t.Fatal(err)
	}
	l.proc("turn-1").finish("first", nil) // settles with session id turn-1
	waitFor(t, "the worker to finish", func() bool { return q.Statuses()[0].State == Done })

	q.Close()

	cleanups := l.cleanups()
	if len(cleanups) != 1 || cleanups[0].session != "turn-1" {
		t.Fatalf("cleanups = %+v, want the settled worker's session turn-1", cleanups)
	}
	if len(q.Statuses()) != 0 {
		t.Errorf("the settled worker still appears after Close: %+v", q.Statuses())
	}
}

// --- unhappy: Close must not clean what it has no session for ---

func TestCloseOnAnEmptyQueueCleansNothing(t *testing.T) {
	l := newCleanRecorder()
	q := New(l, 1, fixedNow)
	q.Close()
	if got := l.cleanups(); len(got) != 0 {
		t.Errorf("cleanups = %+v, want none on an empty queue", got)
	}
}

func TestCloseCancelsARunningWorkerAndCleansItOnce(t *testing.T) {
	l := newCleanRecorder()
	q := New(l, 1, fixedNow)
	if _, err := q.Delegate([]Spec{continueSpec("turn-1", time.Minute)}); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the worker to run", func() bool { return q.Statuses()[0].State == Running })

	q.Close()

	// Close cancels a running worker instead of releasing it: the job stays,
	// marked cancelled, and its session is cleaned once by the cancel path when
	// the killed process exits and reports its id, never a second time.
	waitFor(t, "the cancelled worker's session to be cleaned", func() bool { return len(l.cleanups()) == 1 })
	time.Sleep(50 * time.Millisecond)
	if got := l.cleanups(); len(got) != 1 || got[0].session != "turn-1" {
		t.Errorf("cleanups = %+v, want turn-1 exactly once", got)
	}
	if st := q.Statuses(); len(st) != 1 || st[0].State != Cancelled {
		t.Errorf("statuses = %+v, want the worker kept as cancelled, not released", st)
	}
}

func TestCloseTwiceCleansEachSessionOnce(t *testing.T) {
	l := newCleanRecorder()
	q := New(l, 1, fixedNow)
	if _, err := q.Delegate([]Spec{continueSpec("turn-1", time.Minute)}); err != nil {
		t.Fatal(err)
	}
	l.proc("turn-1").finish("first", nil)
	waitFor(t, "the worker to finish", func() bool { return q.Statuses()[0].State == Done })

	q.Close()
	q.Close()

	if got := l.cleanups(); len(got) != 1 {
		t.Errorf("cleanups = %+v, want exactly one across two Close calls", got)
	}
}

// lateExitProc is a worker whose process keeps running after it is killed
// until the test opens exit, so a test can release a settled (cancelled) job
// before its process exits, the order Close can produce in a real server.
type lateExitProc struct {
	*fakeProc
	exit     chan struct{}
	returned chan struct{}
}

func (p lateExitProc) Wait() error {
	defer close(p.returned)
	<-p.killed
	<-p.exit
	return errors.New("killed")
}

type lateExitLauncher struct{ p lateExitProc }

func (l lateExitLauncher) Launch(Spec) (Process, error) { return l.p, nil }

// TestALateExitAfterReleaseDoesNotPanic proves a worker released between its
// cancel and its process exit does not crash the queue when the exit lands:
// release already dropped the job, so the late exit has nothing to record.
func TestALateExitAfterReleaseDoesNotPanic(t *testing.T) {
	p := lateExitProc{fakeProc: newFakeProc(), exit: make(chan struct{}), returned: make(chan struct{})}
	q := New(lateExitLauncher{p: p}, 1, fixedNow)
	if _, err := q.Delegate([]Spec{spec("run", time.Minute)}); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the worker to run", func() bool { return q.Statuses()[0].State == Running })
	q.Cancel([]string{"w001"})
	if released, _, _ := q.Release(nil); len(released) != 1 {
		t.Fatalf("released = %v, want the cancelled w001", released)
	}
	close(p.exit) // the process exits only now, after its job was released
	<-p.returned
	// The queue records the exit right after Wait returns; give it that moment,
	// then prove the queue still works (a nil job used to panic here).
	time.Sleep(100 * time.Millisecond)
	if len(q.Statuses()) != 0 {
		t.Errorf("statuses = %+v, want the released job gone", q.Statuses())
	}
}

// TestReleaseCleansUpWithTheWorktreeDirForAWriteWorker proves the queue passes
// the worktree directory (not the original spec dir) to the cleaner for a write
// worker, since a write worker's transcript is keyed by the worktree it ran in.
func TestReleaseCleansUpWithTheWorktreeDirForAWriteWorker(t *testing.T) {
	l := newCleanRecorder()
	q := New(l, 1, fixedNow)
	// Delegate a worker and give it a worktree + a session id by hand through the
	// fake proc, then settle it so release can clean it up.
	if _, err := q.Delegate([]Spec{continueSpec("w", time.Minute)}); err != nil {
		t.Fatal(err)
	}
	// Record a worktree on the job the way a write launch would, via the job map.
	q.mu.Lock()
	q.jobs["w001"].worktree = "/repo/.downshift/worktrees/w001"
	q.mu.Unlock()
	l.proc("w").finish("done", nil)
	waitFor(t, "the worker to finish", func() bool { return q.Statuses()[0].State == Done })

	released, _, _ := q.Release([]string{"w001"})
	if len(released) != 1 {
		t.Fatalf("released = %v, want w001", released)
	}
	cleanups := l.cleanups()
	if len(cleanups) != 1 {
		t.Fatalf("cleanups = %+v, want one", cleanups)
	}
	if cleanups[0].spec.Dir != "/repo/.downshift/worktrees/w001" {
		t.Errorf("cleanup dir = %q, want the worktree dir", cleanups[0].spec.Dir)
	}
}

// TestReleaseAllDeletesEverySettledWorker releases with no ids and drops every
// finished worker, leaving a still-running one alone.
func TestReleaseAllDeletesEverySettledWorker(t *testing.T) {
	fakeHome(t)
	l := newFakeLauncher()
	q := New(l, 2, fixedNow)
	if _, err := q.Delegate([]Spec{spec("a", time.Minute), spec("b", time.Minute)}); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "both to run", func() bool { return len(l.started()) == 2 })
	// Finish only the first; the second stays running.
	l.proc("a").finish("done", nil)
	waitFor(t, "a to finish", func() bool { return q.Statuses()[0].State == Done })

	released, _, _ := q.Release(nil)
	if len(released) != 1 || released[0] != "w001" {
		t.Fatalf("released = %v, want only the finished w001", released)
	}
	// w002 is still running and was not dropped.
	left := q.Statuses()
	if len(left) != 1 || left[0].ID != "w002" {
		t.Fatalf("remaining = %+v, want only the running w002", left)
	}
}

// TestCodexReleaseDeletesTheRolloutByIdSuffix proves the codex cleaner finds
// the one date-partitioned rollout file whose name ends with the session id and
// deletes only it, leaving another session's rollout in the same folder.
func TestCodexReleaseDeletesTheRolloutByIdSuffix(t *testing.T) {
	home := fakeHome(t)
	id := "01a18a0d-9ec4-7e83-ade3-da8d519fa474"
	other := "01b99999-0000-0000-0000-000000000000"
	day := filepath.Join(home, ".codex", "sessions", "2026", "10", "29")
	if err := os.MkdirAll(day, 0o755); err != nil {
		t.Fatal(err)
	}
	mine := filepath.Join(day, "rollout-2026-10-29T20-45-10-"+id+".jsonl")
	keep := filepath.Join(day, "rollout-2026-10-29T21-00-00-"+other+".jsonl")
	for _, p := range []string{mine, keep} {
		if err := os.WriteFile(p, []byte("{}\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := codexCleanSession("/any/dir", id); err != nil {
		t.Fatalf("codex clean: %v", err)
	}
	if exists(mine) {
		t.Errorf("the worker's rollout was not deleted: %s", mine)
	}
	if !exists(keep) {
		t.Errorf("another session's rollout was deleted: %s", keep)
	}
}
