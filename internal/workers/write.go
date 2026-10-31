// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package workers

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ErrWriteOff refuses a write job when the write route is not enabled.
var ErrWriteOff = errors.New("write mode is off; the owner enables it with DOWNSHIFT_WORKERS=bypass")

// errNotRepo refuses a write job outside a git repository (spec section 7).
var errNotRepo = errors.New("write mode needs a git repository; the directory is not inside one")

// WriteLaunchers returns the write route: Claude in print mode with the bypass
// permission mode, Codex in exec mode with workspace-write and approval never.
// Both run through writeIsolated, so they start inside their own worktree only.
func WriteLaunchers() map[string]Launcher {
	return map[string]Launcher{
		"claude": writeIsolated{inner: cliLauncher{plan: claudeWrite}},
		"codex":  writeIsolated{inner: cliLauncher{plan: codexWrite}},
	}
}

// Routes sends read jobs to the read launcher and write jobs to the write one.
// A nil write launcher refuses every write job before anything starts.
func Routes(read, write Launcher) Launcher { return routes{read: read, write: write} }

type routes struct {
	read  Launcher
	write Launcher
}

func (r routes) Launch(spec Spec) (Process, error) {
	if !spec.Write {
		return r.read.Launch(spec)
	}
	if r.write == nil {
		return nil, ErrWriteOff
	}
	return r.write.Launch(spec)
}

// WorktreeLauncher wraps inner so a job runs in its own git worktree: it
// creates the worktree, records the base commit, launches inner inside it and
// reports the worktree to the queue (for Diff). It is the composition the write
// route uses, exposed so a caller can wrap any inner launcher the same way.
func WorktreeLauncher(inner Launcher) Launcher { return writeIsolated{inner: inner} }

// writeIsolated gives a write job its own worktree and then launches the inner
// recipe inside it.
type writeIsolated struct {
	inner Launcher
}

func (w writeIsolated) Launch(spec Spec) (Process, error) {
	root, err := gitOut(spec.Dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return nil, errNotRepo
	}
	root = strings.TrimSpace(root)
	// Resolve HEAD to a commit before the worktree is made, so the diff reads
	// against a fixed base even if the worker commits and moves the branch.
	base, err := gitOut(root, "rev-parse", "HEAD")
	if err != nil {
		return nil, errNotRepo
	}
	base = strings.TrimSpace(base)
	excludeWorktrees(root)
	path := filepath.Join(root, ".downshift", "worktrees", spec.ID)
	branch := "downshift/" + spec.ID
	if _, err := gitOut(root, "worktree", "add", "-b", branch, path, "HEAD"); err != nil {
		return nil, fmt.Errorf("could not create the worktree: %v", err)
	}
	spec.Dir = path
	proc, err := w.inner.Launch(spec)
	if err != nil {
		return nil, err
	}
	note := fmt.Sprintf("worktree: %s (branch %s). Review the diff with git before you merge; Downshift does not merge.", path, branch)
	return &noted{Process: proc, note: note, path: path, base: base}, nil
}

// worktreesIgnore is the line that keeps the worktree folder out of git status.
const worktreesIgnore = "/.downshift/worktrees/"

// excludeWorktrees adds the worktree folder to the repository's local exclude
// file, so a job's worktrees do not show up as untracked files in the user's
// checkout. It is local to the clone and never committed. Best effort: if the
// file cannot be written the job still runs, and the folder shows in git status.
func excludeWorktrees(root string) {
	out, err := gitOut(root, "rev-parse", "--git-path", "info/exclude")
	if err != nil {
		return
	}
	path := strings.TrimSpace(out)
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, path)
	}
	current, _ := os.ReadFile(path)
	for _, line := range strings.Split(string(current), "\n") {
		if strings.TrimSpace(line) == worktreesIgnore {
			return
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	prefix := ""
	if len(current) > 0 && !strings.HasSuffix(string(current), "\n") {
		prefix = "\n"
	}
	_, _ = f.WriteString(prefix + worktreesIgnore + "\n")
}

// noted appends the worktree note to the report, whatever the outcome, and
// reports the worktree path and base commit to the queue so Diff can read the
// work without a path from the caller.
type noted struct {
	Process
	note string
	path string
	base string
}

func (n *noted) Report() string { return n.Process.Report() + "\n\n" + n.note }

// worktreeInfo satisfies worktreeReporter: the queue records these when the job
// starts, and Diff reads them.
func (n *noted) worktreeInfo() (string, string) { return n.path, n.base }

// SessionID forwards the inner process's captured session id, so a write worker
// in a worktree is resumable too. noted embeds the Process interface, which
// does not declare SessionID, so this forward is explicit: without it the
// queue's sessionReporter check would miss a write worker's id.
func (n *noted) SessionID() string {
	if sr, ok := n.Process.(sessionReporter); ok {
		return sr.SessionID()
	}
	return ""
}

// gitOut runs git in dir and returns its standard output. GitEnv keeps a hook's
// own GIT_DIR or GIT_INDEX_FILE from pointing git at another repository.
func gitOut(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = GitEnv(os.Environ())
	out, err := cmd.Output()
	return string(out), err
}

// GitEnv returns env without the GIT_* variables that a git hook sets. They
// would make git act on the hook's repository, not the one the job names.
func GitEnv(env []string) []string {
	out := make([]string, 0, len(env))
	for _, kv := range env {
		if !strings.HasPrefix(kv, "GIT_") {
			out = append(out, kv)
		}
	}
	return out
}
