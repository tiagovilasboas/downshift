// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// pluginMarketplaceSource is the GitHub repository that hosts the Downshift
// plugin marketplace (see .claude-plugin/marketplace.json).
const pluginMarketplaceSource = "tiagovilasboas/downshift"

// setupExec is the process boundary of setup. The zero value runs nothing, so
// a caller that does not set it (the unit tests) keeps the settings-hooks path.
type setupExec struct {
	LookPath func(name string) (string, error)
	Run      func(name string, args ...string) ([]byte, error)
}

func realSetupExec() setupExec {
	return setupExec{
		LookPath: exec.LookPath,
		Run: func(name string, args ...string) ([]byte, error) {
			return exec.Command(name, args...).CombinedOutput()
		},
	}
}

// ensurePlugin installs the Downshift plugin through Claude Code's own CLI, so
// the hooks and the bundled cache-tax mod arrive together ("install, it works").
// It reports whether the plugin is, or in a dry run would be, active. Every
// failure falls back to the settings hooks with a note: setup never leaves
// Claude Code without routing because a plugin command failed.
func ensurePlugin(o setupOptions, t setupTarget) (bool, []string) {
	if t.Plugin == "" || o.NoPlugin || o.Exec.Run == nil || o.Exec.LookPath == nil {
		return false, nil
	}
	path := filepath.Join(o.Home, filepath.FromSlash(t.File))
	if raw, err := os.ReadFile(path); err == nil {
		var parsed any
		if json.Unmarshal(raw, &parsed) == nil && pluginEnabled(parsed, t.Plugin) {
			return true, nil // planTarget handles the already-enabled case
		}
	}
	fallback := func(why string) (bool, []string) {
		return false, []string{why + "; wiring the hooks in settings.json instead " +
			"(install the plugin later with `/plugin install downshift --marketplace " + pluginMarketplaceSource + "`)"}
	}
	claude, err := o.Exec.LookPath("claude")
	if err != nil {
		return fallback("`claude` is not on PATH, so the plugin cannot be installed from here")
	}
	// The plugin's hooks run `downshift` by name. A login shell without it on
	// PATH would leave routing off without any error.
	if runtime.GOOS != "windows" {
		shell := os.Getenv("SHELL")
		if shell == "" {
			shell = "/bin/sh"
		}
		out, err := o.Exec.Run(shell, "-lc", "command -v downshift")
		onPath := strings.TrimSpace(string(out))
		if err != nil || onPath == "" {
			return fallback("`downshift` is not on PATH in a login shell, and the plugin's hooks call it by name")
		}
		if !sameFile(onPath, o.Binary) {
			return fallback("`downshift` resolves to " + onPath + " in a login shell, not to this binary (" + o.Binary +
				"), so the plugin's hooks would run a different build")
		}
	}
	addCmd := "claude plugin marketplace add " + pluginMarketplaceSource
	installCmd := "claude plugin install " + t.Plugin
	if o.DryRun {
		return true, []string{"would run `" + addCmd + "` and `" + installCmd + "` (hooks and the cache-tax mod come from the plugin)"}
	}
	if out, err := o.Exec.Run(claude, "plugin", "marketplace", "add", pluginMarketplaceSource); err != nil && !pluginAlreadyDone(out) {
		return fallback("`" + addCmd + "` failed (" + oneLine(out, err) + ")")
	}
	if out, err := o.Exec.Run(claude, "plugin", "install", t.Plugin); err != nil && !pluginAlreadyDone(out) {
		return fallback("`" + installCmd + "` failed (" + oneLine(out, err) + ")")
	}
	return true, []string{"installed the " + t.Plugin + " plugin with `claude plugin install`: the hooks and the cache-tax mod come from it " +
		"(`--no-plugin` keeps them in settings.json instead)"}
}

// sameFile reports whether two paths name the same file once symlinks are
// resolved; a path that cannot be resolved is compared as written.
func sameFile(a, b string) bool {
	resolve := func(p string) string {
		if r, err := filepath.EvalSymlinks(p); err == nil {
			return r
		}
		return filepath.Clean(p)
	}
	return resolve(a) == resolve(b)
}

func pluginAlreadyDone(out []byte) bool {
	s := strings.ToLower(string(out))
	return strings.Contains(s, "already")
}

func oneLine(out []byte, err error) string {
	s := strings.TrimSpace(string(out))
	if s == "" {
		return err.Error()
	}
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	return s
}

// removeOwnHooks deletes the Downshift hooks of t from doc and returns how
// many it removed. When the plugin carries the hooks, leaving the settings copy
// in place would route every spawn twice.
func removeOwnHooks(doc *jobj, t setupTarget) int {
	holder := doc
	for _, key := range t.HooksPath {
		v, ok := holder.get(key)
		if !ok {
			return 0
		}
		next, isObj := asObj(v)
		if !isObj {
			return 0
		}
		holder = next
	}
	removed := 0
	for _, h := range t.Hooks {
		v, ok := holder.get(h.Event)
		if !ok {
			continue
		}
		list, isArr := v.([]any)
		if !isArr {
			continue
		}
		var kept []any
		for _, item := range list {
			entry, isObj := asObj(item)
			if !isObj {
				kept = append(kept, item)
				continue
			}
			if t.Nested {
				inner, _ := entry.vals["hooks"].([]any)
				var rest []any
				for _, hk := range inner {
					if ho, ok := asObj(hk); ok && isDownshiftCommand(hookCommand(ho), h.Args) {
						removed++
						continue
					}
					rest = append(rest, hk)
				}
				if len(inner) > 0 && len(rest) == 0 {
					continue // the group only held Downshift's hook
				}
				if len(rest) != len(inner) {
					entry.set("hooks", rest)
				}
				kept = append(kept, item)
				continue
			}
			if isDownshiftCommand(hookCommand(entry), h.Args) {
				removed++
				continue
			}
			kept = append(kept, item)
		}
		if len(kept) == 0 {
			holder.del(h.Event)
		} else if len(kept) != len(list) || removed > 0 {
			holder.set(h.Event, kept)
		}
	}
	if len(holder.keys) == 0 && holder != doc && len(t.HooksPath) > 0 {
		doc.del(t.HooksPath[0])
	}
	return removed
}

func pluginNotesRemoved(n int) string {
	return fmt.Sprintf("removed %d Downshift hook(s) from settings.json: the plugin carries them, and keeping both would route every spawn twice", n)
}
