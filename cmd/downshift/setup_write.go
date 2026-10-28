// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const setupBackupInfix = ".downshift-bak-"

// setupWrite backs up the existing file, then writes the new content
// atomically. It returns the backup path ("" for a new file; a marker file
// records that the file did not exist so --undo can remove it).
func setupWrite(p setupPlan, stamp string) (string, error) {
	if err := os.MkdirAll(filepath.Dir(p.path), 0o700); err != nil {
		return "", err
	}
	if p.mode != 0 {
		// A generated script runs on every tool call. MkdirAll leaves a directory
		// that already exists as it was, so close it to everyone else here; if we
		// cannot (not ours), fail rather than put a script where others can swap it.
		if err := os.Chmod(filepath.Dir(p.path), 0o700); err != nil {
			return "", fmt.Errorf("cannot restrict %s to its owner: %w", filepath.Dir(p.path), err)
		}
	}
	mode := os.FileMode(0o600)
	bak := p.path + setupBackupInfix + stamp
	if p.existed {
		if st, err := os.Stat(p.path); err == nil {
			mode = st.Mode().Perm()
		}
		if err := os.WriteFile(bak, p.before, mode); err != nil {
			return "", fmt.Errorf("backup: %w", err)
		}
	} else {
		if err := os.WriteFile(bak+".new", nil, 0o600); err != nil {
			return "", fmt.Errorf("backup marker: %w", err)
		}
		bak = ""
	}
	if p.mode != 0 {
		mode = p.mode // a generated script must stay executable
	}
	if err := atomicWriteFile(p.path, p.after, mode); err != nil {
		return "", err
	}
	return bak, nil
}

// latestBackup returns the newest backup for path, and whether it is the
// "file did not exist" marker.
func latestBackup(path string) (backup string, marker bool) {
	matches, _ := filepath.Glob(path + setupBackupInfix + "*")
	if len(matches) == 0 {
		return "", false
	}
	sort.Strings(matches)
	b := matches[len(matches)-1]
	return b, strings.HasSuffix(b, ".new")
}

func setupUndo(o setupOptions, targets []setupTarget) int {
	type restore struct {
		t      setupTarget
		path   string
		backup string
		marker bool
		cur    []byte
		want   []byte
	}
	var todo []restore
	readErr := false
	// whole is true for a file setup generates in full: a copy that lost the
	// ownership marker was edited by someone, so undo leaves it alone.
	addRestore := func(t setupTarget, path string, whole bool) {
		b, marker := latestBackup(path)
		if b == "" {
			fmt.Fprintf(o.Out, "%s: no backup of %s, nothing to undo\n", t.Name, setupDisplay(o.Home, path))
			return
		}
		r := restore{t: t, path: path, backup: b, marker: marker}
		r.cur, _ = os.ReadFile(path)
		if whole && len(r.cur) > 0 && !setupOwns(r.cur) {
			fmt.Fprintf(o.Out, "%s: %s was edited after setup, left as it is\n", t.Name, setupDisplay(o.Home, path))
			return
		}
		if !marker {
			data, err := os.ReadFile(b)
			if err != nil {
				fmt.Fprintf(o.Err, "%s: cannot read backup %s: %v\n", t.Name, b, err)
				readErr = true
				return
			}
			r.want = data
		}
		todo = append(todo, r)
	}
	for _, t := range targets {
		if t.File != "" {
			addRestore(t, filepath.Join(o.Home, filepath.FromSlash(t.File)), false)
		}
		if t.Steering != nil {
			addRestore(t, filepath.Join(o.Home, filepath.FromSlash(t.Steering.Path)), !t.Steering.Managed)
		}
		for _, w := range wrapperPathsFor(o, t) {
			addRestore(t, w, true)
		}
		if t.Registry != "" {
			path := filepath.Join(o.Home, filepath.FromSlash(t.Registry))
			if b, _ := latestBackup(path); b != "" {
				addRestore(t, path, false)
			}
		}
	}
	if readErr {
		return 1
	}
	if len(todo) == 0 {
		return 0
	}
	for _, r := range todo {
		shown := setupDisplay(o.Home, r.path)
		verb := "will restore"
		if o.DryRun {
			verb = "would restore"
		}
		if r.marker {
			fmt.Fprintf(o.Out, "%s: %s %s to absent (setup created it)\n", r.t.Name, shown, verb)
		} else {
			fmt.Fprintf(o.Out, "%s: %s %s from %s\n", r.t.Name, shown, verb, setupDisplay(o.Home, r.backup))
		}
		fmt.Fprint(o.Out, unifiedDiff("a/"+shown, diffName(shown, !r.marker, "b"), string(r.cur), string(r.want)))
		fmt.Fprintln(o.Out)
	}
	if o.DryRun {
		fmt.Fprintln(o.Out, "Dry run: nothing was written.")
		return 0
	}
	if !setupConfirm(o, "Restore these files? [y/N] ") {
		fmt.Fprintln(o.Out, "Not applied. Re-run with --yes to skip this question.")
		return 1
	}
	code := 0
	for _, r := range todo {
		var err error
		if r.marker {
			err = os.Remove(r.path)
			if errors.Is(err, os.ErrNotExist) {
				err = nil
			}
		} else {
			mode := os.FileMode(0o600) // never wider than the backup; no 0644 window
			if st, serr := os.Stat(r.backup); serr == nil {
				mode = st.Mode().Perm()
			}
			err = atomicWriteFile(r.path, r.want, mode)
		}
		if err != nil {
			fmt.Fprintf(o.Err, "%s: restore failed: %v\n", r.t.Name, err)
			code = 1
			continue
		}
		fmt.Fprintf(o.Out, "%s: restored %s\n", r.t.Name, setupDisplay(o.Home, r.path))
	}
	return code
}

// atomicWriteFile replaces path with data without ever exposing a partial or
// wider-than-mode file. A symlinked path is resolved first, so the link stays
// a link and the real file (often in a dotfiles repository) is the one
// replaced. The temporary file is unique, lives next to the target (same
// filesystem for the rename), is synced before the rename, and the directory
// is synced after it so a crash leaves the old or the new content, never an
// empty file.
func atomicWriteFile(path string, data []byte, mode os.FileMode) error {
	target := path
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		target = resolved
	}
	dir := filepath.Dir(target)
	tmp, err := os.CreateTemp(dir, filepath.Base(target)+".downshift-tmp-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	fail := func(err error) error {
		tmp.Close()
		os.Remove(name)
		return err
	}
	if err := tmp.Chmod(mode); err != nil {
		return fail(err)
	}
	if _, err := tmp.Write(data); err != nil {
		return fail(err)
	}
	if err := tmp.Sync(); err != nil {
		return fail(err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(name)
		return err
	}
	if err := os.Rename(name, target); err != nil {
		os.Remove(name)
		return err
	}
	if d, err := os.Open(dir); err == nil {
		_ = d.Sync()
		d.Close()
	}
	return nil
}
