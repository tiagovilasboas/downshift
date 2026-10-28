// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package archtest

import (
	"os/exec"
	"slices"
	"strings"
	"testing"
)

const modulePath = "github.com/tiagovilasboas/downshift"

// externalExceptions lists the import paths under internal/ that may import a
// module outside the standard library, each with its reason. It is empty: every
// package under internal/ uses the standard library only, and the third-party
// modules the binary links enter through cmd/downshift (terminal styling). A
// new entry is a deliberate decision, for example an MCP library behind a single
// transport package (docs/internal/plans/mcp-delegation-spec.md, R1).
var externalExceptions = map[string]string{}

// externalModules returns the import paths in deps that are neither the
// standard library nor this module, sorted and without duplicates. deps is the
// output of `go list -deps -f '{{if not .Standard}}{{.ImportPath}}{{end}}'`.
func externalModules(deps []string) []string {
	var out []string
	for _, d := range deps {
		d = strings.TrimSpace(d)
		if d == "" || d == modulePath || strings.HasPrefix(d, modulePath+"/") {
			continue
		}
		out = append(out, d)
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// TestInternalPackagesUseOnlyTheStandardLibrary backs the claim made in the
// docs: the decision, hook, compactor, playbook and every other internal
// package use the Go standard library only. "No dependencies" would be false for
// the binary as a whole, so the claim is scoped to what this test checks.
func TestInternalPackagesUseOnlyTheStandardLibrary(t *testing.T) {
	out, err := exec.Command("go", "list", "-deps", "-f", "{{if not .Standard}}{{.ImportPath}}{{end}}", modulePath+"/internal/...").Output()
	if err != nil {
		t.Fatalf("go list: %v", err)
	}
	lines := strings.Split(string(out), "\n")
	if !slices.Contains(lines, modulePath+"/internal/core") {
		t.Fatalf("go list did not report internal/core; the check would pass on nothing:\n%s", out)
	}
	ext := externalModules(lines)
	for _, e := range ext {
		if _, ok := externalExceptions[e]; !ok {
			t.Errorf("internal/ reaches the external module %s; keep internal/ on the standard library, or add a reasoned entry to externalExceptions", e)
		}
	}
}

func TestExternalModulesFiltersTheStandardLibraryAndThisModule(t *testing.T) {
	got := externalModules([]string{
		"", modulePath, modulePath + "/internal/core",
		"charm.land/lipgloss/v2", "golang.org/x/sys/unix", "charm.land/lipgloss/v2",
	})
	want := []string{"charm.land/lipgloss/v2", "golang.org/x/sys/unix"}
	if !slices.Equal(got, want) {
		t.Fatalf("externalModules = %v, want %v", got, want)
	}
}
