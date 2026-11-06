// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package archtest

import (
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// walkFiles returns the slash paths (relative to the repository root) of every
// file with the given suffix, skipping hidden directories, node_modules and
// vendor. Sorted, so failures are reported in a stable order.
func walkFiles(t *testing.T, suffix string) []string {
	t.Helper()
	var out []string
	err := filepath.WalkDir(repoRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if d.IsDir() {
			hidden := strings.HasPrefix(name, ".") || name == "node_modules" || name == "vendor"
			if path != repoRoot && hidden {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(name, suffix) {
			return nil
		}
		rel, err := filepath.Rel(repoRoot, path)
		if err != nil {
			return err
		}
		out = append(out, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(out)
	return out
}

// allProductionGoFiles is walkFiles(".go") minus tests, testdata fixtures and the
// package being removed.
func allProductionGoFiles(t *testing.T) []string {
	t.Helper()
	var out []string
	for _, p := range walkFiles(t, ".go") {
		isTest := strings.HasSuffix(p, "_test.go")
		isFixture := strings.Contains(p, "/testdata/")
		if isTest || isFixture {
			continue
		}
		out = append(out, p)
	}
	return out
}
