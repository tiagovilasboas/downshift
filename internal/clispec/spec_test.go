// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package clispec

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

func TestNamesAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, c := range table {
		if seen[c.Name] {
			t.Errorf("duplicate command %q", c.Name)
		}
		seen[c.Name] = true
		if c.Summary == "" {
			t.Errorf("%q has no summary", c.Name)
		}
		if c.AliasOf != "" {
			if _, ok := Lookup(c.AliasOf); !ok {
				t.Errorf("%q aliases unknown %q", c.Name, c.AliasOf)
			}
		}
	}
}

func TestSuggestableSkipsHiddenAndLegacy(t *testing.T) {
	got := Suggestable()
	for _, bad := range []string{"exec", "shell-init", "docs-manifest", "models list", "claude-code-post-tool-use"} {
		if slices.Contains(got, bad) {
			t.Errorf("%q must not be suggested", bad)
		}
	}
	if !slices.Contains(got, "doctor") {
		t.Error("doctor must be suggested")
	}
}

// Every DOWNSHIFT_* literal in production code is listed, and nothing else.
func TestEnvVarsMatchSource(t *testing.T) {
	re := regexp.MustCompile(`"(DOWNSHIFT_[A-Z0-9_]+)"`)
	found := map[string]bool{}
	for _, root := range []string{"../../internal", "../../cmd"} {
		_ = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
				return nil
			}
			b, _ := os.ReadFile(p)
			for _, m := range re.FindAllStringSubmatch(string(b), -1) {
				found[m[1]] = true
			}
			return nil
		})
	}
	listed := EnvVars()
	if !slices.IsSorted(listed) {
		t.Error("EnvVars must be sorted")
	}
	for name := range found {
		if !slices.Contains(listed, name) {
			t.Errorf("%s is read by the code but missing from EnvVars", name)
		}
	}
	for _, name := range listed {
		if !found[name] {
			t.Errorf("%s is listed but no production code reads it", name)
		}
	}
}
