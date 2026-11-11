// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package workers

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/tiagovilasboas/downshift-ai/internal/lazyre"
)

// A worker's session is persisted so a later turn can resume it (T1), which
// writes a transcript to disk keyed by the working directory. A read worker
// runs in the directory the caller named, so its transcript lands in that
// project's folder and would show in the user's own /resume there. This file is
// the counterpart that keeps the "nothing lingers" property: when the queue is
// done with a worker's session (release, cancel, server close), it deletes the
// transcript the Downshift created. The deletion stays per-CLI inside each
// launcher (the same owner split as the launch and the resume): the queue only
// asks, it never learns where a CLI stores a session.
//
// Security is the whole point of this file. A session id comes from the CLI's
// own output, but it is still only validated and then used to build a path, so
// every deletion:
//   - refuses an id that is not a plain token (no "/", "..", "~", "*", or any
//     path separator), so it can never name a path outside the session root;
//   - resolves the CLI's session root under the user's home and confirms the
//     target stays inside that resolved root after cleaning;
//   - never follows a symlink: a target (or a matched file) that is a symlink
//     is left alone, so a planted link cannot redirect the delete;
//   - removes only the exact file (and, for Claude, the session-env directory
//     the CLI keys by that same id), never a wide glob and never a recursive
//     wipe of a project directory;
//   - treats a missing file as success (idempotent), and returns an error only
//     when a present file could not be removed, which the caller reports as a
//     warning without failing the release.

// sessionCleaner is the optional contract a Launcher satisfies when it can
// delete a session its CLI persisted. The queue calls it for every session the
// Downshift created, passing the original spec (for the working directory the
// transcript is keyed by) and the opaque session id; the launcher owns where
// its CLI stores the session and how to delete it safely. A Launcher that does
// not implement it is simply a no-op cleanup: nothing to delete, no error.
type sessionCleaner interface {
	CleanupSession(spec Spec, sessionID string) error
}

// cleanSession is the per-CLI deletion an argv carries, built by the planner
// next to the report, tokens, session and resume functions. It is given the
// worker's directory (the transcript is keyed by it) and the opaque session id,
// already checked non-empty by the queue; it validates the id's shape itself
// and deletes only what its CLI wrote. nil means the recipe persists nothing to
// delete.
type cleanSessionFunc func(dir, sessionID string) error

// CleanupSession deletes the session the given worker persisted, through the
// launcher of its agent, so a ByAgent queue cleans up whenever the agent's
// launcher can. An agent with no launcher, or one whose launcher cannot clean
// up, is a no-op: there is nothing this queue persisted for it to delete. This
// keeps the cleanup agnostic at the queue, with the per-CLI path inside each
// launcher (the same owner split as Launch and LaunchResume).
func (m byAgent) CleanupSession(spec Spec, sessionID string) error {
	l, ok := m[spec.Agent]
	if !ok {
		return nil
	}
	sc, ok := l.(sessionCleaner)
	if !ok {
		return nil
	}
	return sc.CleanupSession(spec, sessionID)
}

// CleanupSession deletes the session this launcher's CLI persisted for a worker.
// It builds the argv from the original spec (the same planner the launch used)
// only to reach the per-CLI cleanup function; a recipe that persists nothing
// (cleanSession nil) is a no-op. The spec's directory and the opaque id are all
// the cleanup needs; it never starts a process.
func (l cliLauncher) CleanupSession(spec Spec, sessionID string) error {
	a, err := l.plan(spec)
	if err != nil {
		return nil // a spec that cannot even plan has started no session to clean
	}
	if a.cleanup != nil {
		a.cleanup() // the planner's own temp files (e.g. codex's report file), if any
	}
	if a.cleanSession == nil {
		return nil
	}
	return a.cleanSession(spec.Dir, sessionID)
}

// sessionIDPattern is the shape a session id must have before it is ever used
// to build a path: letters, digits, dot, underscore and dash, one to a hundred
// and twenty-eight of them, starting with a letter or digit. It admits the UUID
// Claude writes and the thread id Codex writes, and rejects anything with a
// path separator, a parent reference, a tilde or a glob metacharacter, so a
// validated id can only ever name a leaf inside a known root.
var sessionIDPattern = lazyre.New(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

// validSessionID reports whether id is safe to put in a path. A "." or ".."
// anywhere, a separator, or a glob character fails the pattern; this is the one
// gate every cleaner passes an id through before building a path.
func validSessionID(id string) bool {
	if id == "" || strings.Contains(id, "..") {
		return false
	}
	return sessionIDPattern.MatchString(id)
}

// userConfigRoot resolves <home>/<rel> to a real, symlink-free absolute path,
// the root a CLI stores its sessions under (e.g. ".claude/projects"). It
// returns "" when the home cannot be found or the root does not exist, so a
// cleaner with no root to search simply does nothing. Resolving the root once,
// with symlinks followed here and nowhere below, is what lets every target be
// checked for containment against a stable real path.
func userConfigRoot(rel string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	root := filepath.Join(home, filepath.FromSlash(rel))
	real, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "" // the root does not exist (or is unreadable): nothing to clean
	}
	return real
}

// removeLeafUnderRoot deletes a single named leaf (a file or an empty-keyed
// directory) directly inside root, with every safety check: the name must be a
// single clean path element, the resolved target must stay inside root, and the
// target must not be a symlink (it is never followed). A missing target is
// success. recursive allows removing a directory the CLI keys by the session id
// (Claude's session-env/<id>), still only after the target has been proved to
// be that exact element under root and not a symlink.
func removeLeafUnderRoot(root, name string, recursive bool) error {
	if root == "" {
		return nil
	}
	// The name must be exactly one path element: Base of a Clean of it is itself,
	// and it is not "." or "..". This rejects any separator or parent reference
	// that slipped past the id check (defence in depth) before a path is built.
	if name == "" || name != filepath.Base(filepath.Clean(name)) || name == "." || name == ".." {
		return nil
	}
	target := filepath.Join(root, name)
	// Containment: the target must sit directly inside root. Join already cleaned
	// it; confirm the prefix so nothing climbs out even if a check above changes.
	if filepath.Dir(target) != root {
		return nil
	}
	info, err := os.Lstat(target)
	if errors.Is(err, fs.ErrNotExist) {
		return nil // already gone: a delete is idempotent
	}
	if err != nil {
		return err
	}
	// Never follow a symlink: a planted link where the CLI file should be must
	// not redirect the delete to another path. Remove the link itself only.
	if info.Mode()&fs.ModeSymlink != 0 {
		return os.Remove(target)
	}
	if info.IsDir() {
		if !recursive {
			return nil // a file was expected, not a directory: leave it
		}
		return os.RemoveAll(target)
	}
	return os.Remove(target)
}

// claudeSessionRel encodes a working directory the way Claude names its project
// folder under ~/.claude/projects: every rune that is not a letter or digit
// becomes a dash (so "/Users/t/.downshift" becomes "-Users-t--downshift"),
// confirmed against the installed CLI 2.1.316. The transcript of a session is
// then <that folder>/<session_id>.jsonl, and the CLI also keeps a
// session-env/<session_id> directory keyed by the id alone.
func claudeProjectFolder(dir string) string {
	abs, err := filepath.Abs(dir)
	if err != nil {
		abs = dir
	}
	var b strings.Builder
	for _, r := range abs {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
	}
	return b.String()
}

// claudeCleanSession deletes the transcript Claude persisted for a worker, and
// the session-env directory the CLI keyed by the same id. Both are reached by
// the exact session id (validated first), inside the resolved ~/.claude roots,
// with no symlink followed and no glob. A missing file is success.
func claudeCleanSession(dir, sessionID string) error {
	if !validSessionID(sessionID) {
		return nil
	}
	var firstErr error
	keep := func(err error) {
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}
	projects := userConfigRoot(".claude/projects")
	if projects != "" {
		folder := filepath.Join(projects, claudeProjectFolder(dir))
		// The transcript lives one level deeper, in the dir-keyed project folder.
		// That folder is named from the worker's own directory, not from the id,
		// so it is a known element under the resolved projects root; delete the
		// single <id>.jsonl inside it, never the folder itself.
		if filepath.Dir(folder) == projects {
			keep(removeLeafUnderRoot(folder, sessionID+".jsonl", false))
		}
	}
	// session-env/<id> is keyed by the id alone: delete that one directory.
	keep(removeLeafUnderRoot(userConfigRoot(".claude/session-env"), sessionID, true))
	return firstErr
}

// codexCleanSession deletes the rollout file Codex persisted for a worker.
// Codex stores sessions date-partitioned under ~/.codex/sessions as
// rollout-<timestamp>-<thread_id>.jsonl (or .jsonl.zst), confirmed against
// codex-cli 0.163.0 (the filename's id suffix equals the thread_id the
// thread.started event reports). The id does not give the date path, so the
// one file is found by a bounded walk of that single root, matching only a
// basename whose id suffix is exactly this session, skipping any symlinked
// directory and never following a symlinked match. A missing file is success.
func codexCleanSession(_ string, sessionID string) error {
	if !validSessionID(sessionID) {
		return nil
	}
	root := userConfigRoot(".codex/sessions")
	if root == "" {
		return nil
	}
	suffixes := []string{"-" + sessionID + ".jsonl", "-" + sessionID + ".jsonl.zst"}
	var firstErr error
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // unreadable entry: skip it, do not fail the whole walk
		}
		if d.IsDir() {
			// Never descend into a symlinked directory: the walk stays inside the
			// real sessions tree only.
			if d.Type()&fs.ModeSymlink != 0 {
				return filepath.SkipDir
			}
			return nil
		}
		name := d.Name()
		if !strings.HasPrefix(name, "rollout-") {
			return nil
		}
		matched := false
		for _, s := range suffixes {
			if strings.HasSuffix(name, s) {
				matched = true
				break
			}
		}
		if !matched {
			return nil
		}
		// A matched file must not be a symlink: remove the entry only if it is a
		// regular file where the rollout should be.
		info, lerr := os.Lstat(path)
		if lerr != nil {
			return nil
		}
		if info.Mode()&fs.ModeSymlink != 0 {
			if rerr := os.Remove(path); rerr != nil && firstErr == nil {
				firstErr = rerr
			}
			return nil
		}
		if rerr := os.Remove(path); rerr != nil && firstErr == nil {
			firstErr = rerr
		}
		return nil
	})
	return firstErr
}
