// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package setup

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/tiagovilasboas/downshift-ai/mods"
)

// planMods plans the target's mods as skills-dir plugins: the embedded folder
// written under SkillsDir/<name>, which Claude Code loads as <name>@skills-dir.
// The files are ordinary setup plans, so the diff, the backup and --undo apply,
// and the mod matches this binary without a network fetch. A mod already
// enabled from the marketplace is left to it, so it never loads twice.
func planMods(o Options, t Target) []Plan {
	if len(t.Mods) == 0 || t.SkillsDir == "" || o.NoMods {
		return nil
	}
	root := filepath.Join(o.Home, filepath.FromSlash(t.SkillsDir))
	var plans []Plan
	for _, name := range t.Mods {
		dir := filepath.Join(root, name)
		if id := name + "@" + pluginMarketplace(t); modEnabled(o, t, id) {
			plans = append(plans, Plan{Target: t, Path: dir, Skip: "the " + id + " plugin already carries the mod"})
			continue
		}
		files, err := modFiles(name)
		if err != nil {
			return append(plans, Plan{Target: t, Path: dir, Err: err})
		}
		conflict := modFolderConflict(dir, name)
		for _, f := range files {
			p := Plan{Target: t, Path: filepath.Join(root, filepath.FromSlash(f)), Loaded: true}
			body, rerr := mods.FS.ReadFile(f)
			switch {
			case rerr != nil:
				p.Err = rerr
			case conflict != "":
				p.Err = fmt.Errorf("%s", conflict)
			default:
				if cur, cerr := os.ReadFile(p.Path); cerr == nil {
					p.Before, p.Existed = cur, true
				}
				p.After = body
			}
			plans = append(plans, p)
		}
	}
	return plans
}

// modFiles lists a mod's shipped files. Its tests and test fixtures stay out,
// and so do the types Claude Code generates into a loaded mod's folder, which a
// local build would otherwise embed.
func modFiles(name string) ([]string, error) {
	var out []string
	err := fs.WalkDir(mods.FS, name, func(p string, d fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return err
		case d.IsDir() && p == name+"/.claude-plugin/types":
			return fs.SkipDir
		case !d.IsDir() && !isModTestFile(path.Base(p)):
			out = append(out, p)
		}
		return nil
	})
	return out, err
}

// modPath is a mod file setup backed up, and how many folders above it are
// the mod's own (removed by --undo once empty).
type modPath struct {
	path  string
	depth int
}

// modPathsFor lists the target's mod files that have a setup backup.
func modPathsFor(o Options, t Target) []modPath {
	if t.SkillsDir == "" {
		return nil
	}
	var out []modPath
	for _, name := range t.Mods {
		files, _ := modFiles(name)
		for _, f := range files {
			p := filepath.Join(o.Home, filepath.FromSlash(t.SkillsDir), filepath.FromSlash(f))
			if b, _ := o.LatestBackup(p); b != "" {
				out = append(out, modPath{path: p, depth: strings.Count(f, "/")})
			}
		}
	}
	return out
}

func isModTestFile(base string) bool {
	return strings.Contains(base, ".test.") || strings.Contains(base, ".fixture.")
}

// modFolderConflict returns why dir cannot take the mod, or "". A missing or
// empty folder is free; one whose manifest names the mod is ours to update.
func modFolderConflict(dir, name string) string {
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) == 0 {
		return ""
	}
	raw, err := os.ReadFile(filepath.Join(dir, ".claude-plugin", "plugin.json"))
	var manifest struct {
		Name string `json:"name"`
	}
	if err == nil && json.Unmarshal(raw, &manifest) == nil && manifest.Name == name {
		return ""
	}
	return dir + " exists and is not the " + name + " mod; move it aside and run setup again"
}

// pluginMarketplace is the marketplace half of the target's plugin id.
func pluginMarketplace(t Target) string {
	if i := strings.LastIndexByte(t.Plugin, '@'); i >= 0 {
		return t.Plugin[i+1:]
	}
	return ""
}

func modEnabled(o Options, t Target, id string) bool {
	raw, err := os.ReadFile(filepath.Join(o.Home, filepath.FromSlash(t.File)))
	if err != nil {
		return false
	}
	var parsed any
	return json.Unmarshal(raw, &parsed) == nil && PluginEnabled(parsed, id)
}
