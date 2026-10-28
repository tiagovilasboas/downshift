// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/tiagovilasboas/downshift/skills"
)

// installedSkills lists the skills setup installs, by folder name in skills.FS.
var installedSkills = []string{skills.Audit}

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
func ownedMarkdown(body string) string {
	if strings.HasPrefix(body, "---\n") {
		if i := strings.Index(body[4:], "\n---\n"); i >= 0 {
			cut := 4 + i + len("\n---\n")
			return body[:cut] + setupOwnedNote + body[cut:]
		}
	}
	return setupOwnedNote + body
}

// skillPathsFor lists the skill files setup keeps for t, so --undo can remove
// them. It is empty for a harness without a skills directory.
func skillPathsFor(o setupOptions, t setupTarget) []string {
	if t.SkillsDir == "" {
		return nil
	}
	var out []string
	for _, name := range installedSkills {
		files, _ := skillFiles(name)
		for _, f := range files {
			out = append(out, filepath.Join(o.Home, filepath.FromSlash(t.SkillsDir), filepath.FromSlash(f)))
		}
	}
	return out
}

// planSkills plans every skill file for a target that declares a SkillsDir.
// It reuses setupPlan, so the diff, the backup and --undo behave like any
// other generated file. A skill folder that exists without our marker is a
// conflict: nothing is written into it.
func planSkills(o setupOptions, t setupTarget) []setupPlan {
	if t.SkillsDir == "" {
		return nil
	}
	root := filepath.Join(o.Home, filepath.FromSlash(t.SkillsDir))
	if o.PluginActive[t.Name] {
		return []setupPlan{{target: t, path: root, skip: "the " + t.Plugin + " plugin already carries the Downshift skills"}}
	}
	var plans []setupPlan
	for _, name := range installedSkills {
		files, err := skillFiles(name)
		if err != nil {
			return []setupPlan{{target: t, path: filepath.Join(root, name), err: err}}
		}
		conflict := skillFolderConflict(filepath.Join(root, name))
		for _, f := range files {
			body, err := skills.FS.ReadFile(f)
			p := setupPlan{target: t, path: filepath.Join(root, filepath.FromSlash(f))}
			switch {
			case err != nil:
				p.err = err
			case conflict != "":
				p.err = fmt.Errorf("%s", conflict)
			default:
				if cur, rerr := os.ReadFile(p.path); rerr == nil {
					p.before, p.existed = cur, true
				}
				p.after = []byte(ownedMarkdown(string(body)))
			}
			plans = append(plans, p)
		}
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
	if cur, err := os.ReadFile(filepath.Join(dir, "SKILL.md")); err == nil && setupOwns(cur) {
		return ""
	}
	return dir + " exists and was not installed by downshift setup; move it aside and run setup again"
}
