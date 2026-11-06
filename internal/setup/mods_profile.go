// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package setup

import (
	"os"
	"path/filepath"
	"strings"
)

// pluginDirsVar is the Claude Code variable that loads plugin folders by path.
const pluginDirsVar = "CLAUDE_CODE_PLUGIN_DIRS"

// shellProfiles are the startup files a POSIX shell reads; on Windows none
// exists, so nothing is planned there.
var shellProfiles = []string{".zshrc", ".zprofile", ".zshenv", ".bashrc", ".bash_profile", ".profile"}

// planModProfiles removes a mod's folder from a CLAUDE_CODE_PLUGIN_DIRS export
// in the shell profiles, since setup now installs the mod and a second copy
// would load it twice. Each change is an ordinary plan: shown in the diff,
// backed up, and restored by --undo. The running process's variable cannot be
// changed, so a value set elsewhere gets a note instead.
func planModProfiles(o Options, t Target) []Plan {
	if len(t.Mods) == 0 || o.NoMods {
		return nil
	}
	var plans []Plan
	for _, name := range shellProfiles {
		p := filepath.Join(o.Home, name)
		raw, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		after, changed := dropModDirs(string(raw), t.Mods)
		if !changed {
			continue
		}
		if st, lerr := os.Lstat(p); lerr == nil && st.Mode()&os.ModeSymlink != 0 {
			// A linked profile usually lives in a dotfiles repository: say what to
			// change rather than write through the link.
			plans = append(plans, Plan{Target: t, Path: p,
				Skip: Display(o.Home, p) + " is a link; remove the " + strings.Join(t.Mods, ", ") +
					" folder from its " + pluginDirsVar + " line yourself, or the mod loads twice"})
			continue
		}
		plans = append(plans, Plan{Target: t, Path: p, Before: raw, Existed: true, After: []byte(after),
			Notes: []string{"removes the " + strings.Join(t.Mods, ", ") + " folder from " + pluginDirsVar +
				": setup installs the mod, and loading it from both would run it twice (open a new terminal after)"}})
	}
	if len(plans) == 0 && envNamesMod(os.Getenv(pluginDirsVar), t.Mods) {
		plans = append(plans, Plan{Target: t, Path: pluginDirsVar,
			Skip: pluginDirsVar + " names a " + strings.Join(t.Mods, ", ") + " folder but no shell profile sets it; " +
				"remove it where it is set, or the mod loads twice"})
	}
	return plans
}

// modProfilePathsFor lists the shell profiles setup backed up for this target.
func modProfilePathsFor(o Options, t Target) []string {
	if len(t.Mods) == 0 {
		return nil
	}
	var out []string
	for _, name := range shellProfiles {
		p := filepath.Join(o.Home, name)
		if b, _ := o.LatestBackup(p); b != "" {
			out = append(out, p)
		}
	}
	return out
}

// dropModDirs rewrites every `export CLAUDE_CODE_PLUGIN_DIRS=...` line without
// the mods' folders. A line left empty goes, with a comment just above it that
// names a mod.
func dropModDirs(text string, names []string) (string, bool) {
	lines := strings.Split(text, "\n")
	var out []string
	changed := false
	for _, line := range lines {
		value, ok := pluginDirsValue(line)
		if !ok || !envNamesMod(value, names) {
			out = append(out, line)
			continue
		}
		changed = true
		var kept []string
		for _, dir := range strings.Split(value, string(os.PathListSeparator)) {
			if dir != "" && !isModDir(dir, names) {
				kept = append(kept, dir)
			}
		}
		if len(kept) > 0 {
			out = append(out, "export "+pluginDirsVar+`="`+strings.Join(kept, string(os.PathListSeparator))+`"`)
			continue
		}
		if n := len(out); n > 0 && strings.HasPrefix(strings.TrimSpace(out[n-1]), "#") && mentionsMod(out[n-1], names) {
			out = out[:n-1]
		}
	}
	return strings.Join(out, "\n"), changed
}

// pluginDirsValue returns the unquoted value of an export of the variable.
func pluginDirsValue(line string) (string, bool) {
	s := strings.TrimSpace(line)
	s = strings.TrimPrefix(s, "export ")
	rest, ok := strings.CutPrefix(strings.TrimSpace(s), pluginDirsVar+"=")
	if !ok {
		return "", false
	}
	return strings.Trim(strings.TrimSpace(rest), `"'`), true
}

func envNamesMod(value string, names []string) bool {
	for _, dir := range strings.Split(value, string(os.PathListSeparator)) {
		if dir != "" && isModDir(dir, names) {
			return true
		}
	}
	return false
}

func isModDir(dir string, names []string) bool {
	base := filepath.Base(filepath.Clean(dir))
	for _, n := range names {
		if base == n {
			return true
		}
	}
	return false
}

func mentionsMod(line string, names []string) bool {
	for _, n := range names {
		if strings.Contains(line, n) {
			return true
		}
	}
	return false
}
