// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tiagovilasboas/downshift/internal/paths"
)

// setupPlan is the outcome of planning one target.
type setupPlan struct {
	target  setupTarget
	path    string
	before  []byte // nil when the file does not exist
	existed bool
	after   []byte
	notes   []string
	err     error
	// mode is the permission of a file setup creates or rewrites; 0 keeps the
	// default (the existing file's mode, or 0600 for a new one).
	mode os.FileMode
	// skip is why nothing is planned for this target (not an error).
	skip string
	// companions are files the config points at (a generated wrapper). They are
	// written first, and the config is not written if one of them fails.
	companions []setupPlan
}

func (p setupPlan) changed() bool {
	return p.err == nil && p.skip == "" && (!p.existed || string(p.before) != string(p.after))
}

// stateDir is where setup keeps the files it generates (the wrappers).
func (o setupOptions) stateDir() string {
	if o.StateDir != "" {
		return o.StateDir
	}
	return filepath.Join(o.Home, paths.DirName)
}

// setupSelect resolves which targets to act on: the named ones, or every
// harness whose config directory exists.
func setupSelect(o setupOptions) ([]setupTarget, int) {
	byName := map[string]setupTarget{}
	var all []string
	for _, t := range setupTargets {
		byName[t.Name] = t
		all = append(all, t.Name)
	}
	var out []setupTarget
	if len(o.Names) > 0 {
		seen := map[string]bool{}
		for _, n := range o.Names {
			t, ok := byName[n]
			if !ok {
				fmt.Fprintf(o.Err, "setup: unknown harness %q (known: %s)\n", n, strings.Join(all, ", "))
				return nil, 2
			}
			if seen[n] {
				continue
			}
			seen[n] = true
			if !isDir(filepath.Join(o.Home, t.ConfigDir)) {
				fmt.Fprintf(o.Err, "setup: %s not found: ~/%s does not exist\n", n, t.ConfigDir)
				return nil, 1
			}
			out = append(out, t)
		}
		return out, 0
	}
	for _, t := range setupTargets {
		if isDir(filepath.Join(o.Home, t.ConfigDir)) {
			out = append(out, t)
		}
	}
	if len(out) == 0 {
		var dirs []string
		for _, t := range setupTargets {
			dirs = append(dirs, "~/"+t.ConfigDir)
		}
		fmt.Fprintf(o.Err, "setup: no supported harness found (looked for %s)\n", strings.Join(dirs, ", "))
		return nil, 1
	}
	return out, 0
}

func isDir(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

// planTarget reads the file and computes the merged document. It never writes.
func planTarget(o setupOptions, t setupTarget) setupPlan {
	p := setupPlan{target: t, path: filepath.Join(o.Home, filepath.FromSlash(t.File))}
	raw, err := os.ReadFile(p.path)
	switch {
	case err == nil:
		p.existed = true
		p.before = raw
	case errors.Is(err, os.ErrNotExist):
		if t.RequireFile {
			p.skip = fmt.Sprintf("%s does not exist, so %s is not set up yet; nothing was written", setupDisplay(o.Home, p.path), t.Name)
			return p
		}
	default:
		p.err = err
		return p
	}
	doc, err := parseJSONDoc(raw)
	if err != nil {
		p.err = fmt.Errorf("not valid JSON, left untouched: %w", err)
		return p
	}
	// With the plugin enabled, the plugin already carries the hooks: writing
	// them here too would route every spawn twice. Only the session policy
	// and the statusline (settings, not hooks) are still applied.
	var pluginNote []string
	if t.Plugin != "" {
		var parsed any
		enabled := json.Unmarshal(raw, &parsed) == nil && pluginEnabled(parsed, t.Plugin)
		if enabled || o.PluginActive[t.Name] {
			if removed := removeOwnHooks(doc, t); removed > 0 {
				pluginNote = append(pluginNote, pluginNotesRemoved(removed))
			}
			t.Hooks = nil
			if enabled {
				pluginNote = append(pluginNote, "plugin "+t.Plugin+" is enabled and carries the hooks, so none were written here "+
					"(session policy and statusline still apply); `/plugin uninstall "+t.Plugin+"` and rerun setup with --no-plugin to use settings hooks instead")
			}
		}
	}
	// A wrapper hook already registered from another directory (the repository
	// checkout the install guide points at) is the user's: leave it, and write
	// no wrapper for it. Anything else gets the generated wrapper, written
	// before the config that points at it.
	var wrapperNotes []string
	var kept []setupHook
	for _, h := range t.Hooks {
		if h.Wrapper == "" {
			kept = append(kept, h)
			continue
		}
		want := wrapperPath(o.stateDir(), h)
		if cur := existingWrapperCommand(doc, t, h); cur != "" && cur != want {
			wrapperNotes = append(wrapperNotes, fmt.Sprintf("%s is already wired through %s: left untouched", h.Event, cur))
			continue
		}
		kept = append(kept, h)
		w := planWrapper(o, h)
		w.target = t
		if w.err != nil {
			p.err = w.err
			return p
		}
		p.companions = append(p.companions, w)
	}
	t.Hooks = kept
	notes, err := mergeTarget(doc, t, o.Binary, o.stateDir(), o.NoSessionPolicy, o.YouShouldKnow)
	if err != nil {
		p.err = err
		return p
	}
	p.notes = append(pluginNote, append(append(wrapperNotes, notes...), t.Notes...)...)
	out, err := renderJSONDoc(doc)
	if err != nil {
		p.err = err
		return p
	}
	p.after = out
	return p
}
