// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package setup

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/tiagovilasboas/downshift-ai/internal/setup/configfile"
	"github.com/tiagovilasboas/downshift-ai/skills"
)

// installedSkills lists the skills setup installs, by folder name in skills.FS.
var installedSkills = []string{skills.Audit, skills.Orchestrator, skills.Effort, skills.Squad, skills.Ponytail}

// skillFiles returns the files of one embedded skill, as paths relative to the
// skills directory ("downshift-audit/SKILL.md").
func skillFiles(name string) ([]string, error) {
	var out []string
	err := fs.WalkDir(skills.FS, name, func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			out = append(out, p)
		}
		return err
	})
	return out, err
}

// ownedMarkdown returns body with the ownership note placed after its
// frontmatter, or first when there is none.
// A CRLF body (a Windows checkout without the LF rule) is read the same way.
func ownedMarkdown(body string) string {
	for _, nl := range []string{"\n", "\r\n"} {
		open, close := "---"+nl, nl+"---"+nl
		if !strings.HasPrefix(body, open) {
			continue
		}
		if i := strings.Index(body[len(open):], close); i >= 0 {
			cut := len(open) + i + len(close)
			return body[:cut] + configfile.OwnedNote + body[cut:]
		}
	}
	return configfile.OwnedNote + body
}

// skillPathsFor lists the skill files setup wrote for t, so --undo can remove
// them. "Wrote" is read from the disk: every file setup writes leaves a backup
// (or a "did not exist" marker), so a file without one was never ours. This does
// not depend on what the plugin carries today, which can differ from what it
// carried when setup ran. It is empty for a harness without a skills directory.
func skillPathsFor(o Options, t Target) []string {
	if t.SkillsDir == "" {
		return nil
	}
	var out []string
	for _, name := range installedSkills {
		files, _ := skillFiles(name)
		for _, f := range files {
			p := filepath.Join(o.Home, filepath.FromSlash(t.SkillsDir), filepath.FromSlash(f))
			if b, _ := o.LatestBackup(p); b != "" {
				out = append(out, p)
			}
		}
	}
	return out
}

func isPluginActive(o Options, t Target) bool {
	if o.PluginActive != nil && o.PluginActive[t.Name] {
		return true
	}
	if t.Plugin == "" || t.File == "" {
		return false
	}
	raw, err := os.ReadFile(filepath.Join(o.Home, filepath.FromSlash(t.File)))
	if err != nil {
		return false
	}
	var parsed any
	return json.Unmarshal(raw, &parsed) == nil && PluginEnabled(parsed, t.Plugin)
}

// planSkills plans every skill file for a target that declares a SkillsDir.
// It reuses Plan, so the diff, the backup and --undo behave like any
// other generated file. A skill folder that exists without our marker is a
// conflict: nothing is written into it.
//
// With the target's plugin active, only the skills the plugin lacks are planned:
// the plugin is pinned to the commit it was installed from and can be older
// than this binary. When the plugin's files cannot be read, nothing is planned
// (skills_plugin.go).
func planSkills(o Options, t Target) []Plan {
	if t.SkillsDir == "" {
		return nil
	}
	root := filepath.Join(o.Home, filepath.FromSlash(t.SkillsDir))
	var carried map[string]bool
	if isPluginActive(o, t) {
		var unknown string
		if carried, unknown = pluginCarriedSkills(o, t); unknown != "" {
			return []Plan{{Target: t, Path: root, Skip: unknown}}
		}
	}
	var plans []Plan
	var provided []string
	for _, name := range installedSkills {
		if carried[name] {
			provided = append(provided, name)
			continue
		}
		files, err := skillFiles(name)
		if err != nil {
			return []Plan{{Target: t, Path: filepath.Join(root, name), Err: err}}
		}
		plans = append(plans, planSkillFiles(t, root, name, files)...)
	}
	if len(provided) > 0 {
		skip := Plan{Target: t, Path: root, Skip: carriedReason(t, provided)}
		plans = append([]Plan{skip}, plans...)
	}
	return plans
}

// carriedReason says which skills setup leaves to the plugin.
func carriedReason(t Target, provided []string) string {
	if len(provided) == len(installedSkills) {
		return "the " + t.Plugin + " plugin already carries the Downshift skills"
	}
	return "the " + t.Plugin + " plugin already carries " + strings.Join(provided, ", ") +
		"; the other Downshift skills are installed here because the plugin lacks them"
}

// planSkillFiles plans the files of one skill folder.
func planSkillFiles(t Target, root, name string, files []string) []Plan {
	var plans []Plan
	conflict := skillFolderConflict(filepath.Join(root, name))
	for _, f := range files {
		body, err := skills.FS.ReadFile(f)
		p := Plan{Target: t, Path: filepath.Join(root, filepath.FromSlash(f)), Loaded: true}
		switch {
		case err != nil:
			p.Err = err
		case conflict != "":
			p.Err = fmt.Errorf("%s", conflict)
		default:
			if cur, rerr := os.ReadFile(p.Path); rerr == nil {
				p.Before, p.Existed = cur, true
			}
			p.After = []byte(ownedMarkdown(string(body)))
		}
		plans = append(plans, p)
	}
	return plans
}

// skillFolderConflict returns why dir cannot take our files, or "". A missing
// or empty folder is free; a folder is ours only when its SKILL.md carries the
// ownership marker.
func skillFolderConflict(dir string) string {
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) == 0 {
		return ""
	}
	if cur, err := os.ReadFile(filepath.Join(dir, "SKILL.md")); err == nil && configfile.Owns(cur) {
		return ""
	}
	return dir + " exists and was not installed by downshift setup; move it aside and run setup again"
}
