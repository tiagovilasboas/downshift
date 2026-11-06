// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package setup

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/tiagovilasboas/downshift/hooks"
	"github.com/tiagovilasboas/downshift/internal/setup/configfile"
)

// HarnessFile is one file setup writes verbatim into a harness-owned
// directory. It exists for harnesses whose glue is a program the harness loads
// from a folder (an OpenCode plugin) instead of a JSON hook list. Dest is
// relative to HOME; Source is the path inside FS.
type HarnessFile struct {
	Dest   string
	FS     fs.FS
	Source string
}

// OpenCodeBridgeFiles lists the plugin entry and each module it imports; all
// land in the one plugin folder OpenCode loads.
func OpenCodeBridgeFiles() []HarnessFile {
	var files []HarnessFile
	for _, src := range append([]string{hooks.OpenCodePlugin}, hooks.OpenCodeModules...) {
		files = append(files, HarnessFile{
			Dest:   ".config/opencode/plugins/downshift/" + path.Base(src),
			FS:     hooks.FS,
			Source: src,
		})
	}
	return files
}

// PlanHarnessFiles plans the target's harness files: a whole file setup owns.
// A file setup did not write is a conflict and is left untouched; an existing
// one that carries the ownership marker is refreshed, so an upgraded binary
// replaces its own bridge.
func PlanHarnessFiles(o Options, t Target) []Plan {
	var plans []Plan
	for _, f := range t.Files {
		dest := filepath.Join(o.Home, filepath.FromSlash(f.Dest))
		p := Plan{Target: t, Path: dest, Loaded: true}
		// A linked path usually lives in a versioned repository (dotfiles):
		// setup never writes through it. The path stays unresolved so the plan,
		// a dry run included, reports the link instead of its target.
		if lerr := ErrLinkedPath(dest, o.LinkBases()...); lerr != nil {
			p.Skip = lerr.Error()
			plans = append(plans, p)
			continue
		}
		body, err := fs.ReadFile(f.FS, f.Source)
		switch {
		case err != nil:
			p.Err = err
			plans = append(plans, p)
			continue
		}
		cur, rerr := os.ReadFile(dest)
		switch {
		case rerr == nil:
			p.Before, p.Existed = cur, true
			if !configfile.Owns(cur) {
				p.Err = fmt.Errorf("%s exists and was not written by downshift setup; move it aside and run setup again", Display(o.Home, dest))
				plans = append(plans, p)
				continue
			}
		case !errors.Is(rerr, os.ErrNotExist):
			p.Err = rerr
			plans = append(plans, p)
			continue
		}
		p.After = body
		plans = append(plans, p)
	}
	return plans
}

// HarnessFilePath is a bridge file setup wrote, and how many folders above it
// setup created (removed by --undo once empty).
type HarnessFilePath struct {
	Path  string
	Depth int
}

// HarnessFilePathsFor lists the target's bridge files that have a setup
// backup. A file without one was never ours.
func HarnessFilePathsFor(o Options, t Target) []HarnessFilePath {
	var out []HarnessFilePath
	root := filepath.Join(o.Home, filepath.FromSlash(t.ConfigDir))
	for _, f := range t.Files {
		// Unresolved, like the root: the folder depth counts only what setup
		// created below the config dir, even when the home is behind a link.
		path := filepath.Join(o.Home, filepath.FromSlash(f.Dest))
		if b, _ := o.LatestBackup(path); b == "" {
			continue
		}
		rel := strings.TrimPrefix(filepath.ToSlash(path), filepath.ToSlash(root)+"/")
		out = append(out, HarnessFilePath{Path: path, Depth: strings.Count(rel, "/")})
	}
	return out
}
