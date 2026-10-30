// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/tiagovilasboas/downshift/internal/catalog"
	"github.com/tiagovilasboas/downshift/internal/routing/policy"
)

// newAgentMode is the mode of an agent file Downshift creates. An existing
// file keeps the mode it has.
const newAgentMode os.FileMode = 0o644

// profileNameRegex is the last check before a profile name becomes a file name.
// The policy loader has already validated the name; this keeps the file system
// boundary safe on its own.
var profileNameRegex = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]{0,63}$`)

// agentPlan is one agent file, fully decided and checked, not yet written.
type agentPlan struct {
	name, path, shown, after, modelID string
	before                            []byte
	existed                           bool
	mode                              os.FileMode
}

// unchanged reports that the file already holds what the plan would write.
func (a agentPlan) unchanged() bool { return a.existed && string(a.before) == a.after }

func agentFile(base, name string) string {
	return filepath.Join(base, ".claude", "agents", name+".md")
}

// planAgent decides what the file for one profile must contain and checks that
// it can be written. It reads, never writes: the caller writes only after every
// profile has a plan.
func planAgent(opts policyApplyOptions, root string, cat *catalog.Catalog, pr policy.Profile) (agentPlan, error) {
	name := string(pr.Name)
	if !profileNameRegex.MatchString(name) {
		return agentPlan{}, fmt.Errorf("profile %q: invalid name", name)
	}
	coreTier, ok := pr.Tier.Core()
	if !ok {
		return agentPlan{}, fmt.Errorf("profile %q: unknown tier %q", name, pr.Tier)
	}
	model := cat.ModelFor(opts.Harness, coreTier)
	if model.ID == "" {
		return agentPlan{}, fmt.Errorf("profile %q: the catalog has no %s model for %s", name, pr.Tier, opts.Harness)
	}

	base, target := root, agentFile(root, name)
	if opts.Home != "" {
		if user := agentFile(opts.Home, name); !exists(target) && exists(user) {
			base, target = opts.Home, user
		}
	}
	shown := displayPath(root, opts.Home, target)

	fi, err := inspectAgentTarget(base, target)
	if err != nil {
		return agentPlan{}, fmt.Errorf("profile %q: %w", name, err)
	}
	plan := agentPlan{name: name, path: target, shown: shown, modelID: model.ID, mode: newAgentMode}
	if fi != nil {
		raw, err := os.ReadFile(target)
		if err != nil {
			return agentPlan{}, fmt.Errorf("profile %q: cannot read %s: %w", name, target, err)
		}
		plan.before, plan.existed, plan.mode = raw, true, fi.Mode().Perm()
	}

	description := strings.TrimSpace(pr.Description)
	if description == "" && !hasFrontmatter(string(plan.before)) {
		return agentPlan{}, fmt.Errorf("profile %q: cannot write %s without a description (Claude Code does not load an agent without one); add \"description\" to this profile in routing.profiles", name, shown)
	}
	plan.after = renderAgent(string(plan.before), name, model.ID, description)
	return plan, nil
}

// inspectAgentTarget refuses a target Downshift would write through a symlink or
// over something that is not a regular file. It returns nil info when the file
// does not exist yet. Only the .claude directory, the agents directory and the
// file are examined, below base: a symlinked project path such as /tmp on macOS
// is not the target's business.
func inspectAgentTarget(base, target string) (fs.FileInfo, error) {
	for _, dir := range []string{filepath.Join(base, ".claude"), filepath.Dir(target)} {
		fi, err := os.Lstat(dir)
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		if fi.Mode()&fs.ModeSymlink != 0 {
			return nil, symlinkRefusal(dir)
		}
		if !fi.IsDir() {
			return nil, fmt.Errorf("%s is not a directory", dir)
		}
	}
	fi, err := os.Lstat(target)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if fi.Mode()&fs.ModeSymlink != 0 {
		return nil, symlinkRefusal(target)
	}
	if !fi.Mode().IsRegular() {
		return nil, fmt.Errorf("%s is not a regular file", target)
	}
	return fi, nil
}

func symlinkRefusal(path string) error {
	return fmt.Errorf("%s is a symlink; refusing to write through it (replace it with a regular file or directory, or remove it)", path)
}

// displayPath shows a path relative to the project root, else relative to HOME.
func displayPath(root, home, p string) string {
	if rel, err := filepath.Rel(root, p); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return filepath.ToSlash(rel)
	}
	return setupDisplay(home, p)
}
