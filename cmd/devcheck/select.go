// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"

	"github.com/tiagovilasboas/downshift-ai/internal/lazyre"
)

// gitOut runs git with args and returns its standard output.
func gitOut(args ...string) (string, error) {
	out, err := exec.Command("git", args...).Output()
	return string(out), err
}

// mergeBase is the commit the change is measured from: where HEAD left origin/main.
func mergeBase() (string, error) {
	out, err := gitOut("merge-base", "origin/main", "HEAD")
	if err != nil {
		return "", fmt.Errorf("no origin/main to compare with (run git fetch origin, or make check-full): %w", err)
	}
	return strings.TrimSpace(out), nil
}

// changedFiles lists every file that differs from base: committed, staged,
// unstaged and untracked (not ignored), sorted and unique.
func changedFiles(base string) ([]string, error) {
	tracked, err := gitOut("diff", "--name-only", base)
	if err != nil {
		return nil, err
	}
	untracked, err := gitOut("ls-files", "--others", "--exclude-standard")
	if err != nil {
		return nil, err
	}
	set := map[string]bool{}
	for _, f := range strings.Split(tracked+untracked, "\n") {
		if f = strings.TrimSpace(f); f != "" {
			set[f] = true
		}
	}
	files := make([]string, 0, len(set))
	for f := range set {
		files = append(files, f)
	}
	sort.Strings(files)
	return files, nil
}

// selectPackages asks scripts/ci-test-packages.sh, the selection CI uses, which
// packages the files need: the changed packages and their dependents, "./..."
// when a file cannot be narrowed, none when no Go test applies.
func selectPackages(files []string) ([]string, error) {
	cmd := exec.Command("bash", "scripts/ci-test-packages.sh")
	cmd.Env = append(os.Environ(), "CI_CHANGED_FILES="+strings.Join(files, "\n"))
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("scripts/ci-test-packages.sh: %w", err)
	}
	return strings.Fields(string(out)), nil
}

// wideFiles matches the files ci-test-packages.sh never narrows: go.mod, go.sum,
// scripts, workflows, web and any file at the repository root.
var wideFiles = lazyre.New(`^(go\.mod|go\.sum|scripts/|\.github/|web/)|^[^/]+$`)

// explainSelection says in one line why pkgs were chosen for files.
func explainSelection(files, pkgs []string) string {
	if len(pkgs) != 1 || pkgs[0] != "./..." {
		return fmt.Sprintf("%d changed file(s), %d package(s) with dependents", len(files), len(pkgs))
	}
	var wide []string
	for _, f := range files {
		if wideFiles.MatchString(f) && !strings.HasSuffix(f, ".md") {
			wide = append(wide, f)
		}
	}
	if len(wide) == 0 {
		return "all packages: a changed file has no Go package above it"
	}
	if len(wide) > 3 {
		wide = wide[:3]
	}
	return "all packages: cannot narrow " + strings.Join(wide, ", ")
}
