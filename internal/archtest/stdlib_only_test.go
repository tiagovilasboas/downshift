// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package archtest

import (
	"os/exec"
	"slices"
	"strings"
	"testing"
)

const modulePath = "github.com/tiagovilasboas/downshift-ai"

// externalAllowed lists the internal packages that may import a module outside
// the standard library, and which module prefixes they may import. Every other
// package under internal/ uses the standard library only; the third-party
// modules the binary links enter through cmd/downshift (terminal styling) and
// through the one transport package of the MCP server, which wraps the official
// Go SDK of the protocol (docs/internal/plans/mcp-delegation-spec.md, R1).
var externalAllowed = map[string][]string{
	modulePath + "/internal/mcp/transport": {"github.com/modelcontextprotocol/go-sdk/"},
}

// Packages under consoleTree may import consoleModules too (see allowances),
// except the standard-library-only ones in consoleStdlibOnly.
const consoleTree = modulePath + "/internal/console"

// consoleStdlibOnly are the console packages that stay on the standard library:
// the command runner and the console/plain decision.
var consoleStdlibOnly = []string{consoleTree + "/shared/command", consoleTree + "/surface"}

// consoleModules are the terminal UI modules the console packages may import.
var consoleModules = []string{"charm.land/bubbletea/v2", "charm.land/lipgloss/v2"}

// allowances returns externalAllowed plus the console prefix rule: every
// package under internal/console/ may import the terminal UI modules, except
// consoleStdlibOnly. It is a prefix rule so new console packages need no entry.
func allowances(pkgs map[string][]string) map[string][]string {
	out := map[string][]string{}
	for k, v := range externalAllowed {
		out[k] = v
	}
	for pkg := range pkgs {
		if (pkg == consoleTree || strings.HasPrefix(pkg, consoleTree+"/")) && !slices.Contains(consoleStdlibOnly, pkg) {
			out[pkg] = consoleModules
		}
	}
	return out
}

// edgeOnly lists packages that carry an external module and so may be imported
// only from outside internal/ (that is, from cmd/): importing one from another
// internal package would pull the module into the core.
var edgeOnly = []string{modulePath + "/internal/mcp/transport", modulePath + "/internal/console"}

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
	pkgs := internalImports(t)
	allowed := allowances(pkgs)
	for pkg, imports := range pkgs {
		for _, bad := range externalImports(pkg, imports, allowed) {
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

func TestConsolePrefixRuleKeepsStdlibOnlyPackagesStrict(t *testing.T) {
	pkgs := map[string][]string{
		consoleTree + "/newfeature":       nil,
		consoleTree + "/shared/command":   nil,
		consoleTree + "/surface":          nil,
		modulePath + "/internal/core":     nil,
		modulePath + "/internal/console2": nil,
	}
	got := allowances(pkgs)
	if len(got[consoleTree+"/newfeature"]) == 0 {
		t.Error("a new console package must be allowed the UI modules")
	}
	for _, p := range []string{consoleTree + "/shared/command", consoleTree + "/surface", modulePath + "/internal/core", modulePath + "/internal/console2"} {
		if len(got[p]) != 0 {
			t.Errorf("%s must stay on the standard library", p)
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

// TestConsoleSurfaceStaysPure keeps the console/plain decision free of the UI
// and keeps the routing core and the adapters from depending on it.
func TestConsoleSurfaceStaysPure(t *testing.T) {
	surface := modulePath + "/internal/console/surface"
	pkgs := internalImports(t)
	if _, ok := pkgs[surface]; !ok {
		t.Fatalf("go list did not report %s; the check would pass on nothing", surface)
	}
	if slices.Contains(pkgs[surface], modulePath+"/internal/console") {
		t.Errorf("%s imports internal/console; the decision must not depend on the UI", surface)
	}
	for pkg, imports := range pkgs {
		inCore := pkg == modulePath+"/internal/core"
		inAdapter := strings.HasPrefix(pkg, modulePath+"/internal/adapters/")
		if (inCore || inAdapter) && slices.Contains(imports, surface) {
			t.Errorf("%s imports %s; policy and adapters must not depend on the console decision", pkg, surface)
		}
	}
}
