// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package workers

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// maxDiffBytes caps the diff a worker's worktree returns. It sits below the MCP
// response cap (64 KiB, internal/mcp/tools/info_envelope.go maxResultBytes) so
// the diff, the worker state and the envelope's own fields fit one result.
const maxDiffBytes = 48_000

// errUnknownJob is returned for an id the queue never issued.
var errUnknownJob = errors.New("no worker has that id")

// errNoWorktree is returned for a worker that has no worktree: a read-only job,
// or one whose write worktree was never recorded.
var errNoWorktree = errors.New("that worker has no worktree: only a write worker produces a diff")

// errWorktreeGone is returned when a recorded worktree no longer exists on disk.
var errWorktreeGone = errors.New("the worker's worktree no longer exists")

// DiffResult is one worker's work so far, read from its worktree without
// touching its index. Text is a unified diff: the tracked changes against the
// base commit the worktree was created from, then every untracked file as an
// addition. Truncated says the text was cut at the byte cap; DroppedBytes is
// how much was removed.
type DiffResult struct {
	Job          string `json:"job"`
	State        State  `json:"state"`
	Text         string `json:"diff"`
	Truncated    bool   `json:"truncated"`
	DroppedBytes int    `json:"dropped_bytes,omitempty"`
}

// Diff returns the unified diff a worker produced in its own worktree, plus the
// worker's state. It reads only inside that worktree, never a path from the
// caller. An unknown id, or a worker with no worktree, is a clear error, never
// a panic. A finished worker still returns its diff while the worktree exists.
func (q *Queue) Diff(id string) (DiffResult, error) {
	q.mu.Lock()
	j, ok := q.jobs[id]
	if !ok {
		q.mu.Unlock()
		return DiffResult{}, errUnknownJob
	}
	path, base, state := j.worktree, j.base, j.state
	q.mu.Unlock()
	if path == "" || base == "" {
		return DiffResult{}, errNoWorktree
	}
	if info, err := os.Stat(path); err != nil || !info.IsDir() {
		return DiffResult{}, errWorktreeGone
	}
	text, err := worktreeDiff(path, base)
	if err != nil {
		return DiffResult{}, err
	}
	capped, dropped := capDiff(text)
	return DiffResult{Job: id, State: state, Text: capped, Truncated: dropped > 0, DroppedBytes: dropped}, nil
}

// worktreeDiff builds the unified diff of a worktree against base: the tracked
// changes, then every untracked file rendered as an addition. It never writes
// the index (no `git add`): tracked changes come from `git diff base`, and each
// untracked file from `git diff --no-index /dev/null <file>`, which only reads.
// Binary content is named, never dumped.
func worktreeDiff(dir, base string) (string, error) {
	var out bytes.Buffer
	tracked, err := gitDiffOut(dir, "diff", "--no-color", base)
	if err != nil {
		return "", fmt.Errorf("could not read the worktree diff: %v", err)
	}
	out.WriteString(tracked)
	others, err := gitOut(dir, "ls-files", "--others", "--exclude-standard", "-z")
	if err != nil {
		return "", fmt.Errorf("could not list new files: %v", err)
	}
	for _, rel := range splitZ(others) {
		if rel == "" {
			continue
		}
		if out.Len() > 0 && !strings.HasSuffix(out.String(), "\n") {
			out.WriteByte('\n')
		}
		out.WriteString(untrackedDiff(dir, rel))
	}
	return out.String(), nil
}

// untrackedDiff renders one new file as an addition. A binary file is named,
// not dumped. git exits non-zero when the files differ, which is the whole
// point here, so its output is used whatever the exit code; an empty output
// means git could not read the file, and the file is named as added without a
// body.
func untrackedDiff(dir, rel string) string {
	if isBinary(filepath.Join(dir, rel)) {
		return fmt.Sprintf("diff --git a/%s b/%s\nnew file (binary content omitted)\n", rel, rel)
	}
	out, _ := gitDiffAllowExit(dir, "diff", "--no-color", "--no-index", "/dev/null", rel)
	if strings.TrimSpace(out) == "" {
		return fmt.Sprintf("diff --git a/%s b/%s\nnew file %s (empty or unreadable)\n", rel, rel, rel)
	}
	return out
}

// capDiff keeps the first maxDiffBytes of a diff, cut on a rune boundary, and
// reports how many bytes it dropped.
func capDiff(s string) (string, int) {
	if len(s) <= maxDiffBytes {
		return s, 0
	}
	cut := maxDiffBytes
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	dropped := len(s) - cut
	return s[:cut] + fmt.Sprintf("\n[cut: %d bytes dropped; the diff is over the %d-byte cap]", dropped, maxDiffBytes), dropped
}

// isBinary reports whether a file holds a NUL byte in its first chunk, the same
// test git uses to decide a file is binary. An unreadable file reads as binary,
// so its content is never dumped.
func isBinary(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return true
	}
	defer f.Close()
	buf := make([]byte, 8000)
	n, _ := f.Read(buf)
	return bytes.IndexByte(buf[:n], 0) >= 0
}

// splitZ splits NUL-separated git output into entries.
func splitZ(s string) []string { return strings.Split(s, "\x00") }

// gitDiffOut runs git diff and fails on a non-zero exit. The GitEnv guard keeps
// a hook's GIT_* variables from pointing git at another repository.
func gitDiffOut(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = GitEnv(os.Environ())
	out, err := cmd.Output()
	return string(out), err
}

// gitDiffAllowExit runs git diff and keeps its output whatever the exit code:
// `git diff --no-index` exits 1 when the files differ, which is expected here.
func gitDiffAllowExit(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = GitEnv(os.Environ())
	out, _ := cmd.Output()
	return string(out), nil
}
