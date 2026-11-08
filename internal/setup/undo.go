// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package setup

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/tiagovilasboas/downshift-ai/internal/routing/policy"
	"github.com/tiagovilasboas/downshift-ai/internal/setup/configfile"
)

func Undo(o Options, targets []Target) int {
	type restore struct {
		t      Target
		path   string
		backup string
		marker bool
		cur    []byte
		want   []byte
		prune  int // folder levels emptied by removing it to drop (a skill, a mod)
	}
	var todo []restore
	readErr := false
	// whole is true for a file setup generates in full: a copy that lost the
	// ownership marker was edited by someone, so undo leaves it alone.
	addRestore := func(t Target, path string, whole bool) {
		b, marker := o.LatestBackup(path)
		if b == "" {
			fmt.Fprintf(o.Out, "%s: no backup of %s, nothing to undo\n", t.Name, Display(o.Home, path))
			return
		}
		if err := ErrLinkedPath(path, o.LinkBases()...); err != nil {
			fmt.Fprintf(o.Out, "%s: %v; nothing was restored there\n", t.Name, err)
			return
		}
		r := restore{t: t, path: path, backup: b, marker: marker}
		r.cur, _ = os.ReadFile(path)
		if whole && len(r.cur) > 0 && !configfile.Owns(r.cur) {
			fmt.Fprintf(o.Out, "%s: %s was edited after setup, left as it is\n", t.Name, Display(o.Home, path))
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
		for _, sk := range skillPathsFor(o, t) {
			n := len(todo)
			addRestore(t, sk, true)
			if len(todo) > n {
				todo[n].prune = 2
			}
		}
		// Mod files carry no ownership marker: the folder's manifest is the claim.
		for _, m := range modPathsFor(o, t) {
			n := len(todo)
			addRestore(t, m.path, false)
			if len(todo) > n {
				todo[n].prune = m.depth
			}
		}
		for _, f := range HarnessFilePathsFor(o, t) {
			n := len(todo)
			addRestore(t, f.Path, false)
			if len(todo) > n {
				todo[n].prune = f.Depth
			}
		}
		for _, p := range modProfilePathsFor(o, t) {
			addRestore(t, p, false)
		}
		if t.Registry != "" {
			path := filepath.Join(o.Home, filepath.FromSlash(t.Registry))
			if b, _ := o.LatestBackup(path); b != "" {
				addRestore(t, path, false)
			}
		}
	}
	if o.ProjectDir != "" {
		projPolicy := policy.ProjectPath(o.ProjectDir)
		if b, _ := o.LatestBackup(projPolicy); b != "" {
			addRestore(policyInitTarget, projPolicy, false)
		}
	}
	if readErr {
		return 1
	}
	if len(todo) == 0 {
		return 0
	}
	for _, r := range todo {
		shown := Display(o.Home, r.path)
		verb := "will restore"
		if o.DryRun {
			verb = "would restore"
		}
		if r.marker {
			fmt.Fprintf(o.Out, "%s: %s %s to absent (setup created it)\n", r.t.Name, shown, verb)
		} else {
			fmt.Fprintf(o.Out, "%s: %s %s from %s\n", r.t.Name, shown, verb, Display(o.Home, r.backup))
		}
		fmt.Fprint(o.Out, configfile.UnifiedDiff("a/"+shown, DiffName(shown, !r.marker, "b"), string(r.cur), string(r.want)))
		fmt.Fprintln(o.Out)
	}
	if o.DryRun {
		fmt.Fprintln(o.Out, "Dry run: nothing was written.")
		return 0
	}
	if !Confirm(o, "Restore these files? [y/N] ") {
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
			if err == nil && r.prune > 0 {
				// The "did not exist" markers are spent; leaving them would keep the
				// folders non-empty and make the next install see a foreign folder.
				o.removeMarkers(r.path)
				configfile.RemoveEmptyDirs(filepath.Dir(r.path), r.prune) // a folder setup made
			}
		} else {
			mode := os.FileMode(0o600) // never wider than the backup; no 0644 window
			if st, serr := os.Stat(r.backup); serr == nil {
				mode = st.Mode().Perm()
			}
			err = configfile.WriteAtomic(r.path, r.want, mode)
		}
		if err != nil {
			fmt.Fprintf(o.Err, "%s: restore failed: %v\n", r.t.Name, err)
			code = 1
			continue
		}
		fmt.Fprintf(o.Out, "%s: restored %s\n", r.t.Name, Display(o.Home, r.path))
	}
	return code
}
