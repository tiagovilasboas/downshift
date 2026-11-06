// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package setup

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tiagovilasboas/downshift/internal/setup/configfile"
)

// Select resolves which targets to act on: the named ones, or every
// harness whose config directory exists.
func Select(o Options) ([]Target, int) {
	byName := map[string]Target{}
	var all []string
	for _, t := range Targets {
		byName[t.Name] = t
		all = append(all, t.Name)
	}
	var out []Target
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
			if !IsDir(filepath.Join(o.Home, t.ConfigDir)) {
				fmt.Fprintf(o.Err, "setup: %s not found: ~/%s does not exist\n", n, t.ConfigDir)
				return nil, 1
			}
			out = append(out, t)
		}
		return out, 0
	}
	for _, t := range Targets {
		if IsDir(filepath.Join(o.Home, t.ConfigDir)) {
			out = append(out, t)
		}
	}
	if len(out) == 0 {
		var dirs []string
		for _, t := range Targets {
			dirs = append(dirs, "~/"+t.ConfigDir)
		}
		fmt.Fprintf(o.Err, "setup: no supported harness found (looked for %s)\n", strings.Join(dirs, ", "))
		return nil, 1
	}
	return out, 0
}

func IsDir(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

// planTarget reads the file and computes the merged document. It never writes.
func planTarget(o Options, t Target) Plan {
	p := Plan{Target: t, Path: filepath.Join(o.Home, filepath.FromSlash(t.File))}
	raw, err := os.ReadFile(p.Path)
	switch {
	case err == nil:
		p.Existed = true
		p.Before = raw
	case errors.Is(err, os.ErrNotExist):
		if t.RequireFile {
			p.Skip = fmt.Sprintf("%s does not exist, so %s is not set up yet; nothing was written", Display(o.Home, p.Path), t.Name)
			return p
		}
	default:
		p.Err = err
		return p
	}
	doc, err := configfile.ParseObject(raw)
	if err != nil {
		p.Err = fmt.Errorf("not valid JSON, left untouched: %w", err)
		return p
	}
	// With the plugin enabled, the plugin already carries the hooks: writing
	// them here too would route every spawn twice. Only the session policy
	// and the statusline (settings, not hooks) are still applied.
	var pluginNote []string
	if len(t.Retired) > 0 {
		retired := Target{HooksPath: t.HooksPath, Nested: t.Nested, Hooks: t.Retired}
		if removed := removeOwnHooks(doc, retired); removed > 0 {
			pluginNote = append(pluginNote, fmt.Sprintf("removed %d retired Downshift hook(s) from %s: their command no longer exists", removed, t.File))
		}
	}
	if t.Plugin != "" {
		var parsed any
		enabled := json.Unmarshal(raw, &parsed) == nil && PluginEnabled(parsed, t.Plugin)
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
	// A hook that names a Wrapper is carried by a generated script, written
	// before the config that points at it.
	for _, h := range t.Hooks {
		if h.Wrapper == "" {
			continue
		}
		w := planWrapper(o, h)
		w.Target = t
		if w.Err != nil {
			p.Err = w.Err
			return p
		}
		p.Companions = append(p.Companions, w)
	}
	notes, err := MergeTarget(doc, t, o.Binary, o.StatePath(), o.NoSessionPolicy, o.YouShouldKnow)
	if err != nil {
		p.Err = err
		return p
	}
	p.Notes = append(pluginNote, append(notes, t.Notes...)...)
	out, err := configfile.RenderObject(doc)
	if err != nil {
		p.Err = err
		return p
	}
	p.After = out
	return p
}
