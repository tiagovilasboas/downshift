// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
}

func (p setupPlan) changed() bool {
	return p.err == nil && (!p.existed || string(p.before) != string(p.after))
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
		if json.Unmarshal(raw, &parsed) == nil && pluginEnabled(parsed, t.Plugin) {
			t.Hooks = nil
			pluginNote = []string{"plugin " + t.Plugin + " is enabled and carries the hooks, so none were written here " +
				"(session policy and statusline still apply); `/plugin uninstall " + t.Plugin + "` and rerun setup to use settings hooks instead"}
		}
	}
	notes, err := mergeTarget(doc, t, o.Binary, o.NoSessionPolicy)
	if err != nil {
		p.err = err
		return p
	}
	p.notes = append(pluginNote, append(notes, t.Notes...)...)
	out, err := renderJSONDoc(doc)
	if err != nil {
		p.err = err
		return p
	}
	p.after = out
	return p
}
