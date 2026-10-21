// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package docscheck_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// pkgPath matches a repository directory written as internal/x/ or cmd/x/,
// or quoted as `internal/x` (a trailing slash or a closing backquote marks a
// path, not prose).
var pkgPath = regexp.MustCompile("\\b((?:internal|cmd)/[a-z0-9_]+(?:/[a-z0-9_]+)*)(?:/|`)")

// The docs that describe the current tree name only directories that exist:
// AGENTS.md as a whole, and the Unreleased section of CHANGELOG.md minus its
// Removed subsections (an Added entry for a package removed before the release
// is drift, not history).
func TestCurrentDocsNameExistingPackages(t *testing.T) {
	root := repoRoot(t)
	docs := map[string]string{}
	agents, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	docs["AGENTS.md"] = string(agents)
	changelog, err := os.ReadFile(filepath.Join(root, "CHANGELOG.md"))
	if err != nil {
		t.Fatal(err)
	}
	unreleased := string(changelog)
	if i := strings.Index(unreleased, "## [Unreleased]"); i >= 0 {
		unreleased = unreleased[i+len("## [Unreleased]"):]
		if j := strings.Index(unreleased, "\n## ["); j >= 0 {
			unreleased = unreleased[:j]
		}
	}
	// A "### Removed" subsection names what is gone on purpose.
	for {
		i := strings.Index(unreleased, "### Removed")
		if i < 0 {
			break
		}
		rest := unreleased[i+len("### Removed"):]
		end := len(rest)
		if j := strings.Index(rest, "\n### "); j >= 0 {
			end = j
		}
		unreleased = unreleased[:i] + rest[end:]
	}
	docs["CHANGELOG.md (Unreleased)"] = unreleased
	for name, text := range docs {
		seen := map[string]bool{}
		for _, m := range pkgPath.FindAllStringSubmatch(text, -1) {
			dir := m[1]
			if seen[dir] {
				continue
			}
			seen[dir] = true
			if fi, err := os.Stat(filepath.Join(root, filepath.FromSlash(dir))); err != nil || !fi.IsDir() {
				t.Errorf("%s names %s/, which does not exist", name, dir)
			}
		}
	}
}
