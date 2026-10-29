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

// externalAllowed lists the internal packages that may import a module outside
// the standard library, and which module prefixes they may import. Every other
// package under internal/ uses the standard library only; the third-party
// modules the binary links enter through cmd/downshift (terminal styling) and
// through the one transport package of the MCP server, which wraps the official
// Go SDK of the protocol (docs/internal/plans/mcp-delegation-spec.md, R1).
var externalAllowed = map[string][]string{
	modulePath + "/internal/mcp/transport": {"github.com/modelcontextprotocol/go-sdk/"},
}

// edgeOnly lists packages that carry an external module and so may be imported
// only from outside internal/ (that is, from cmd/): importing one from another
// internal package would pull the module into the core.
var edgeOnly = []string{modulePath + "/internal/mcp/transport"}

// isExternal reports whether an import path is neither the standard library
// (no dot in its first element) nor this module.
func isExternal(path string) bool {
	if path == modulePath || strings.HasPrefix(path, modulePath+"/") {
		return false
	}
	first, _, _ := strings.Cut(path, "/")
	return strings.Contains(first, ".")
}

// externalImports returns the imports of pkg that are external and not covered
// by pkg's allowance.
func externalImports(pkg string, imports []string, allowed map[string][]string) []string {
	var bad []string
	for _, imp := range imports {
		if !isExternal(imp) {
			continue
		}
		ok := false
		for _, prefix := range allowed[pkg] {
			if strings.HasPrefix(imp, prefix) {
				ok = true
			}
		}
		if !ok {
			bad = append(bad, imp)
		}
	}
	return bad
}

// internalImports returns, for every package under internal/, its direct
// production imports (test files are not part of them).
func internalImports(t *testing.T) map[string][]string {
	t.Helper()
	out, err := exec.Command("go", "list", "-f", `{{.ImportPath}} {{join .Imports " "}}`, modulePath+"/internal/...").Output()
	if err != nil {
		t.Fatalf("go list: %v", err)
	}
	pkgs := map[string][]string{}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		f := strings.Fields(line)
		if len(f) > 0 {
			pkgs[f[0]] = f[1:]
		}
	}
	if _, ok := pkgs[modulePath+"/internal/core"]; !ok {
		t.Fatalf("go list did not report internal/core; the check would pass on nothing:\n%s", out)
	}
	return pkgs
}

// TestInternalPackagesUseOnlyTheStandardLibrary backs the claim made in the
// docs. "No dependencies" would be false for the binary as a whole, so the claim
// is scoped to what this test checks: internal/ imports the standard library
// only, except the packages named in externalAllowed.
func TestInternalPackagesUseOnlyTheStandardLibrary(t *testing.T) {
	for pkg, imports := range internalImports(t) {
		for _, bad := range externalImports(pkg, imports, externalAllowed) {
			t.Errorf("%s imports the external module %s; keep internal/ on the standard library, or add a reasoned entry to externalAllowed", pkg, bad)
		}
	}
}

// TestExternalModulesStayAtTheEdge keeps a package that carries an external
// module from being imported by another internal package.
func TestExternalModulesStayAtTheEdge(t *testing.T) {
	for pkg, imports := range internalImports(t) {
		for _, edge := range edgeOnly {
			if slices.Contains(imports, edge) {
				t.Errorf("%s imports %s, which wraps an external module and may be imported from cmd/ only", pkg, edge)
			}
		}
	}
}

func TestExternalImportsFiltersStdlibThisModuleAndTheAllowance(t *testing.T) {
	pkg := modulePath + "/internal/x"
	allowed := map[string][]string{pkg: {"example.com/lib/"}}
	got := externalImports(pkg, []string{
		"fmt", "encoding/json", modulePath + "/internal/core",
		"example.com/lib/sub", "charm.land/lipgloss/v2", "golang.org/x/sys/unix",
	}, allowed)
	want := []string{"charm.land/lipgloss/v2", "golang.org/x/sys/unix"}
	if !slices.Equal(got, want) {
		t.Fatalf("externalImports = %v, want %v", got, want)
	}
	if len(externalImports(modulePath+"/internal/y", []string{"example.com/lib/sub"}, allowed)) != 1 {
		t.Fatal("an allowance for one package must not cover another")
	}
}
