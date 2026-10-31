// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package workers

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// worktreeProc is a finished worker that reports a worktree, so a Diff test
// exercises the queue's recording and git reading without a real CLI.
type worktreeProc struct {
	path string
	base string
}

func (worktreeProc) Wait() error                      { return nil }
func (worktreeProc) Kill() error                      { return nil }
func (worktreeProc) Report() string                   { return "done" }
func (worktreeProc) Stderr() string                   { return "" }
func (worktreeProc) Tokens() *Tokens                  { return nil }
func (p worktreeProc) worktreeInfo() (string, string) { return p.path, p.base }

// worktreeLauncher hands back a worktreeProc pointed at a path and base the
// test prepared.
type worktreeLauncher struct {
	path string
	base string
}

func (l worktreeLauncher) Launch(Spec) (Process, error) {
	return worktreeProc(l), nil
}

// readProc is a finished read-only worker: no worktree, so Diff must refuse it.
type readProc struct{}

func (readProc) Wait() error     { return nil }
func (readProc) Kill() error     { return nil }
func (readProc) Report() string  { return "done" }
func (readProc) Stderr() string  { return "" }
func (readProc) Tokens() *Tokens { return nil }

type readLauncher struct{}

func (readLauncher) Launch(Spec) (Process, error) { return readProc{}, nil }

// git runs a git command in dir and fails the test on error. It strips the
// caller's GIT_* variables (as GitEnv does in production) so a GIT_DIR set by a
// pre-commit hook does not point git at another repository.
func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(GitEnv(os.Environ()),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out)
}

// repoWithWork makes a git repo with one committed file, records its HEAD as
// the base, then makes a worktree-like state: the tracked file is modified and
// a new untracked file is added. It returns the directory and the base commit.
func repoWithWork(t *testing.T) (dir, base string) {
	t.Helper()
	dir = t.TempDir()
	git(t, dir, "init", "-q")
	if err := os.WriteFile(filepath.Join(dir, "kept.txt"), []byte("one\ntwo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, dir, "add", "kept.txt")
	git(t, dir, "commit", "-q", "-m", "base")
	base = strings.TrimSpace(git(t, dir, "rev-parse", "HEAD"))
	// Modify the tracked file and add an untracked one, without staging either.
	if err := os.WriteFile(filepath.Join(dir, "kept.txt"), []byte("one\nchanged\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "new.txt"), []byte("brand new line\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir, base
}

// queueWith starts one job over the launcher and waits for it to finish, so the
// job's worktree is recorded. It returns the job id.
func queueWith(t *testing.T, l Launcher) (*Queue, string) {
	t.Helper()
	q := New(l, 1, time.Now)
	ids, err := q.Delegate([]Spec{{Agent: "claude", Budget: time.Minute, Write: true}})
	if err != nil {
		t.Fatal(err)
	}
	// Let the goroutine that waits on the finished process settle the job.
	deadline := time.Now().Add(2 * time.Second)
	for {
		q.mu.Lock()
		done := q.jobs[ids[0]].state.Finished()
		q.mu.Unlock()
		if done || time.Now().After(deadline) {
			break
		}
		time.Sleep(time.Millisecond)
	}
	return q, ids[0]
}

// Happy path: the diff names the modified tracked file and the new untracked
// file, and the index is left alone (nothing was staged to read it).
func TestDiffShowsModifiedAndNewFiles(t *testing.T) {
	dir, base := repoWithWork(t)
	q, id := queueWith(t, worktreeLauncher{path: dir, base: base})
	res, err := q.Diff(id)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if res.Job != id || res.State != Done {
		t.Fatalf("Diff job/state = %q/%q, want %q/done", res.Job, res.State, id)
	}
	if !strings.Contains(res.Text, "kept.txt") || !strings.Contains(res.Text, "changed") {
		t.Errorf("diff must show the modified tracked file:\n%s", res.Text)
	}
	if !strings.Contains(res.Text, "new.txt") || !strings.Contains(res.Text, "brand new line") {
		t.Errorf("diff must show the new untracked file:\n%s", res.Text)
	}
	if res.Truncated {
		t.Error("a small diff must not be truncated")
	}
	// The index was not touched: new.txt is still untracked after Diff.
	status := git(t, dir, "status", "--porcelain")
	if !strings.Contains(status, "?? new.txt") {
		t.Errorf("Diff must not stage anything; status:\n%s", status)
	}
}

// Unhappy 1: an id the queue never issued is a clear error, no panic.
func TestDiffUnknownIDErrors(t *testing.T) {
	q := New(readLauncher{}, 1, time.Now)
	if _, err := q.Diff("w999"); err == nil {
		t.Fatal("an unknown id must error")
	}
}

// Unhappy 2: a read-only worker has no worktree, so Diff refuses it clearly.
func TestDiffReadOnlyWorkerHasNoWorktree(t *testing.T) {
	q, id := queueWith(t, readLauncher{})
	_, err := q.Diff(id)
	if err == nil {
		t.Fatal("a read-only worker must have no diff")
	}
	if !strings.Contains(err.Error(), "worktree") {
		t.Fatalf("the error must name the missing worktree, got %q", err)
	}
}

// Unhappy 3: a diff over the byte cap is cut, with the amount dropped reported.
func TestDiffTruncatesOverTheCap(t *testing.T) {
	dir := t.TempDir()
	git(t, dir, "init", "-q")
	if err := os.WriteFile(filepath.Join(dir, "seed.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, dir, "add", "seed.txt")
	git(t, dir, "commit", "-q", "-m", "base")
	base := strings.TrimSpace(git(t, dir, "rev-parse", "HEAD"))
	// A new file well over the cap, as untracked content.
	big := strings.Repeat("a line of diff content that is plain text\n", maxDiffBytes/20)
	if err := os.WriteFile(filepath.Join(dir, "big.txt"), []byte(big), 0o644); err != nil {
		t.Fatal(err)
	}
	q, id := queueWith(t, worktreeLauncher{path: dir, base: base})
	res, err := q.Diff(id)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if !res.Truncated || res.DroppedBytes <= 0 {
		t.Fatalf("a large diff must be truncated with a dropped count, got truncated=%v dropped=%d", res.Truncated, res.DroppedBytes)
	}
	if len(res.Text) > maxDiffBytes+200 {
		t.Fatalf("the cut text is %d bytes, over the cap plus the notice", len(res.Text))
	}
	if !strings.Contains(res.Text, "cut:") {
		t.Error("the cut must be announced in the text")
	}
}

// Safety: Diff reads only inside the worktree. A file outside the worktree,
// even one that differs from a committed copy, never appears: the diff is built
// from git run in the worktree against the base, with no caller path.
func TestDiffReadsNothingOutsideTheWorktree(t *testing.T) {
	dir, base := repoWithWork(t)
	// A secret next to the repo, outside the worktree.
	outside := filepath.Join(filepath.Dir(dir), "SECRET_OUTSIDE.txt")
	if err := os.WriteFile(outside, []byte("do not leak me\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(outside) })
	q, id := queueWith(t, worktreeLauncher{path: dir, base: base})
	res, err := q.Diff(id)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if strings.Contains(res.Text, "do not leak me") || strings.Contains(res.Text, "SECRET_OUTSIDE") {
		t.Fatalf("the diff leaked a file outside the worktree:\n%s", res.Text)
	}
}

// A recorded worktree that was removed from disk is a clear error, no panic.
func TestDiffGoneWorktreeErrors(t *testing.T) {
	dir, base := repoWithWork(t)
	q, id := queueWith(t, worktreeLauncher{path: dir, base: base})
	if err := os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := q.Diff(id); err == nil {
		t.Fatal("a missing worktree must error")
	}
}

// A binary new file is named, its bytes never dumped into the diff.
func TestDiffNamesBinaryWithoutDumping(t *testing.T) {
	dir := t.TempDir()
	git(t, dir, "init", "-q")
	if err := os.WriteFile(filepath.Join(dir, "seed.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, dir, "add", "seed.txt")
	git(t, dir, "commit", "-q", "-m", "base")
	base := strings.TrimSpace(git(t, dir, "rev-parse", "HEAD"))
	if err := os.WriteFile(filepath.Join(dir, "blob.bin"), []byte{0x00, 0x01, 0x02, 0x00, 0xff}, 0o644); err != nil {
		t.Fatal(err)
	}
	q, id := queueWith(t, worktreeLauncher{path: dir, base: base})
	res, err := q.Diff(id)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if !strings.Contains(res.Text, "blob.bin") || !strings.Contains(res.Text, "binary") {
		t.Errorf("a binary new file must be named as binary:\n%s", res.Text)
	}
	if strings.ContainsRune(res.Text, '\x00') {
		t.Error("the diff must not contain the binary bytes")
	}
}
