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
	return &noted{Process: proc, note: fmt.Sprintf("worktree: %s (branch %s). Review the diff with git before you merge; Downshift does not merge.", path, branch)}, nil
}

// noted appends the worktree note to the report, whatever the outcome.
type noted struct {
	Process
	note string
}

func (n *noted) Report() string { return n.Process.Report() + "\n\n" + n.note }

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
