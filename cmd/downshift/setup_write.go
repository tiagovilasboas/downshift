// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
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
	if err := os.MkdirAll(filepath.Dir(p.path), 0o755); err != nil {
		return "", err
	}
	mode := os.FileMode(0o644)
	bak := p.path + setupBackupInfix + stamp
	if p.existed {
		if st, err := os.Stat(p.path); err == nil {
			mode = st.Mode().Perm()
		}
		if err := os.WriteFile(bak, p.before, mode); err != nil {
			return "", fmt.Errorf("backup: %w", err)
		}
	} else {
		if err := os.WriteFile(bak+".new", nil, 0o644); err != nil {
			return "", fmt.Errorf("backup marker: %w", err)
		}
		bak = ""
	}
	tmp := p.path + ".downshift-tmp"
	if err := os.WriteFile(tmp, p.after, mode); err != nil {
		return "", err
	}
	if err := os.Rename(tmp, p.path); err != nil {
		os.Remove(tmp)
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
	for _, t := range targets {
		if t.File == "" {
			continue
		}
		path := filepath.Join(o.Home, filepath.FromSlash(t.File))
		b, marker := latestBackup(path)
		if b == "" {
			fmt.Fprintf(o.Out, "%s: no backup of %s, nothing to undo\n", t.Name, setupDisplay(o.Home, path))
			continue
		}
		r := restore{t: t, path: path, backup: b, marker: marker}
		r.cur, _ = os.ReadFile(path)
		if !marker {
			data, err := os.ReadFile(b)
			if err != nil {
				fmt.Fprintf(o.Err, "%s: cannot read backup %s: %v\n", t.Name, b, err)
				return 1
			}
			r.want = data
		}
		todo = append(todo, r)
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
			err = os.WriteFile(r.path, r.want, 0o644)
			if st, serr := os.Stat(r.backup); serr == nil && err == nil {
				err = os.Chmod(r.path, st.Mode().Perm())
			}
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
