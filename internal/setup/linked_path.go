// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package setup

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// linkedComponent returns the first existing component of path that is a
// link, or "". It walks every component strictly below home down to path
// itself; home and everything above it is not checked, so a linked home or
// /var -> /private/var stays fine. A path outside home is walked from its own
// volume root. The walk stops at the first component that does not exist.
// ModeSymlink and ModeIrregular both count: Go reports a Windows junction or
// mount point as irregular, so one rule covers every OS.
func linkedComponent(home, path string) string {
	home, path = filepath.Clean(home), filepath.Clean(path)
	start := filepath.VolumeName(path) + string(filepath.Separator)
	if withinHome(home, path) {
		start = home
	}
	rel, err := filepath.Rel(start, path)
	if err != nil {
		return ""
	}
	cur := start
	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		if part == "" || part == "." {
			continue
		}
		cur = filepath.Join(cur, part)
		st, err := os.Lstat(cur)
		if err != nil {
			return ""
		}
		if st.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 {
			return cur
		}
	}
	return ""
}

// withinHome reports whether path is home or below it. A child named like
// "..cache" is still inside: only a ".." component leaves home.
func withinHome(home, path string) bool {
	rel, err := filepath.Rel(home, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// linkBases are the roots a setup path may live under: the home, the project
// directory of the project policy, and the state directory. Each path is
// walked from the most specific base that contains it (so a project under a
// linked ~/src is not refused for ~/src). The first base is the home, used to
// show paths.
func (o Options) LinkBases() []string {
	bases := []string{o.Home, o.StatePath()}
	if o.ProjectDir != "" {
		bases = append(bases, o.ProjectDir)
	}
	return bases
}

// ErrLinkedPath is the refusal for a path with a linked component; it names
// the component, never where the link points. A path under none of the bases
// gets the leaf check alone: walking from the volume root would refuse
// ordinary locations such as /tmp, which macOS links to /private/tmp.
func ErrLinkedPath(path string, bases ...string) error {
	link, best := "", ""
	for _, b := range bases {
		if withinHome(b, path) && len(filepath.Clean(b)) > len(best) {
			best = filepath.Clean(b)
		}
	}
	if best != "" {
		link = linkedComponent(best, path)
	} else if st, err := os.Lstat(path); err == nil && st.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 {
		link = path
	}
	if link == "" {
		return nil
	}
	return fmt.Errorf("%s is a link; setup leaves linked files alone (edit the real file by hand, or remove the link)", Display(bases[0], link))
}

// RefuseLinkedPlans skips each plan that would write through a link and stamps
// the bases on the others, so Write repeats the check. Only that file is
// skipped: the other files of the same harness (MCP entry, hooks, skills) are
// still written. A skipped file is not a failure; the user fixes the link.
func RefuseLinkedPlans(bases []string, plans []Plan) []Plan {
	out := make([]Plan, len(plans))
	for i, p := range plans {
		p.Bases = bases
		if p.Err == nil && p.Skip == "" && p.Path != "" {
			if err := ErrLinkedPath(p.Path, bases...); err != nil {
				p.Skip = err.Error()
			}
		}
		out[i] = p
	}
	return out
}
