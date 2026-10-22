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
// path, not prose). Directory names may contain hyphens (e.g. cmd/dsmon-hook).
var pkgPath = regexp.MustCompile("\\b((?:internal|cmd)/[a-z0-9_-]+(?:/[a-z0-9_-]+)*)(?:/|`)")

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

// TestAllPackagesInventoried is the reverse of TestCurrentDocsNameExistingPackages:
// every directory under internal/ and cmd/ must be named in at least one of the
// inventory docs (AGENTS.md or docs/contrib/coverage.md). This closes the gap
// where a package exists but no plan or doc acknowledges it.
//
// A directory is exempt when:
//   - its name is "testdata" or starts with "."
//   - it is nested more than one level under internal/ or cmd/ (e.g. internal/a/b
//     is covered by naming internal/a)
func TestAllPackagesInventoried(t *testing.T) {
	root := repoRoot(t)

	// Build the set of all package paths named in the inventory docs.
	inventoryFiles := []string{
		filepath.Join(root, "AGENTS.md"),
		filepath.Join(root, "docs", "contrib", "coverage.md"),
	}
	named := map[string]bool{}
	for _, f := range inventoryFiles {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("inventory doc missing: %s: %v", f, err)
		}
		for _, m := range pkgPath.FindAllStringSubmatch(string(data), -1) {
			named[m[1]] = true
		}
	}

	// Walk internal/ and cmd/, checking top-level directories only.
	for _, base := range []string{"internal", "cmd"} {
		entries, err := os.ReadDir(filepath.Join(root, base))
		if err != nil {
			t.Fatalf("cannot read %s/: %v", base, err)
		}
		for _, e := range entries {
			if !e.IsDir() || e.Name() == "testdata" || strings.HasPrefix(e.Name(), ".") {
				continue
			}
			pkg := base + "/" + e.Name()
			if !named[pkg] {
				t.Errorf("%s/ exists but is not named in AGENTS.md or docs/contrib/coverage.md", pkg)
			}
		}
	}
}

// TestAgentsMdUnderLineCap fails when AGENTS.md exceeds 200 lines. The cap
// keeps the always-loaded agent context lean; detail belongs in on-demand docs.
func TestAgentsMdUnderLineCap(t *testing.T) {
	const cap = 200
	root := repoRoot(t)
	data, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Count(string(data), "\n")
	if lines > cap {
		t.Errorf("AGENTS.md has %d lines (cap %d); move detail to docs/contrib/ or docs/design/", lines, cap)
	}
}
