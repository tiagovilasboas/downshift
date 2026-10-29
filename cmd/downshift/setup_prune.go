// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"fmt"
	"os"
	"time"
)

// prunePlan is what the cleanup would do. newer names a copy built after this
// binary; when it is set nothing is removed, because the newer build should be
// the one running setup.
type prunePlan struct {
	remove []binaryCopy
	kept   []string // one line per copy left alone, with the reason
	newer  string
	// unstamped is true when this build has no build stamp (a go run or a dev
	// build), so the copies cannot be ordered and nothing is removed.
	unstamped bool
}

// planPrune decides which copies go. A copy is removed when it is older than
// this build (or has no stamp at all) and no config setup writes or already
// has still names its path. Otherwise it is kept, with the reason.
func planPrune(copies []binaryCopy, selfBuilt string, configs [][]byte) prunePlan {
	selfTime, err := time.Parse(time.RFC3339, selfBuilt)
	if err != nil {
		return prunePlan{unstamped: true}
	}
	var p prunePlan
	for _, c := range copies {
		if t, err := time.Parse(time.RFC3339, c.built); err == nil && t.After(selfTime) {
			p.newer = c.path
			return p
		}
	}
	for _, c := range copies {
		if referenced(c.path, configs) {
			p.kept = append(p.kept, fmt.Sprintf("%s: kept, a config still names it", c.path))
			continue
		}
		p.remove = append(p.remove, c)
	}
	return p
}

// referenced reports whether any config bytes name the path.
func referenced(path string, configs [][]byte) bool {
	for _, data := range configs {
		if bytes.Contains(data, []byte(path)) {
			return true
		}
	}
	return false
}

// pruneBinaries lists the other downshift copies on the directories setup was
// given, shows which ones it would remove, and removes them only after a yes.
// It runs after the config writes, so the configs already name this build.
// failed is true only when a removal fails; a declined question is not a failure.
func pruneBinaries(o setupOptions, plans []setupPlan) (failed bool) {
	copies := findBinaryCopies(o.BinDirs, o.Binary)
	if len(copies) == 0 {
		return false
	}
	var configs [][]byte
	for _, p := range plans {
		configs = append(configs, p.before, p.after)
	}
	p := planPrune(copies, buildDate, configs)
	switch {
	case p.unstamped:
		fmt.Fprintln(o.Out, "binaries: this build has no build stamp, so other copies are not compared and nothing is removed.")
		return false
	case p.newer != "":
		fmt.Fprintf(o.Out, "binaries: %s was built after this one. Run `downshift setup` from that copy; nothing was removed.\n\n",
			setupDisplay(o.Home, p.newer))
		return false
	}
	for _, line := range p.kept {
		fmt.Fprintf(o.Out, "binaries: %s\n", line)
	}
	if len(p.remove) == 0 {
		return false
	}
	verb := "will be removed"
	if o.DryRun {
		verb = "would be removed"
	}
	for _, c := range p.remove {
		fmt.Fprintf(o.Out, "binaries: %s (built %s) %s\n", setupDisplay(o.Home, c.path), stampOrNone(c.built), verb)
	}
	fmt.Fprintln(o.Out)
	if o.DryRun {
		return false
	}
	if !setupConfirm(o, "Remove these old downshift copies? [y/N] ") {
		fmt.Fprintln(o.Out, "Old copies kept. Re-run with --yes to remove them.")
		return false
	}
	for _, c := range p.remove {
		if err := os.Remove(c.path); err != nil {
			failed = true
			fmt.Fprintf(o.Err, "binaries: could not remove %s: %v\n", setupDisplay(o.Home, c.path), err)
			continue
		}
		fmt.Fprintf(o.Out, "binaries: removed %s\n", setupDisplay(o.Home, c.path))
	}
	return failed
}

func stampOrNone(built string) string {
	if built == "" {
		return "unknown"
	}
	return built
}
