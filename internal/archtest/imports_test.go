// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package archtest enforces the import graph documented in
// docs/design/architecture.md (and AGENTS.md "Key invariants"). It has no
// production code: the test asks `go list -deps` for the transitive production
// imports of a package and fails when one is forbidden. Test files are not part
// of that graph, which is the point: a core test may import nbtier, the binary
// does not.
//
// Accounting layers (docs/internal/tasks/2026-10-21-accounting-v1, section 2).
// Every surface shows the numbers of accounting.Compute, so the layers are
// enforced here:
//
//	L0 internal/telemetry, internal/execwrap  parse and store events; they must
//	   not import accounting or api (rules table, closure check). telemetry
//	   must not import core either: router values reach it through
//	   telemetry/fromcore, the one telemetry package that imports core.
//	L2 internal/accounting/formula            pure formula; imports only catalog
//	   (rules table plus TestLayerDirectImports).
//	L3 internal/accounting                    Compute; its direct imports are
//	   telemetry, execwrap, catalog and accounting/formula, and its closure never
//	   reaches api or an adapter (TestLayerDirectImports, rules table).
//	L4 internal/api, cmd/downshift report.go and sessionpanel.go  surfaces; they
//	   must import accounting (TestSurfacesImportAccounting) and must not add up
//	   telemetry fields themselves: no `WorkSaved +`, `CacheSaved +` or
//	   `SavedBytes +` arithmetic outside the allowlist of TestNoSurfaceArithmetic.
//	   That allowlist holds the one Compute implementation (compute.go) and the
//	   legacy aggregations A6 deletes; an entry that stops matching fails, so the
//	   list can only shrink.
//
// The numbers themselves are held equal by the parity test in cmd/downshift
// (accounting_parity_test.go) over the golden logs in testdata/parity.
package archtest

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"
)

const module = "github.com/tiagovilasboas/downshift/internal/"

// rule: the production dependency closure of pkg must not contain any of
// forbidden (each matches the package itself and its sub-packages).
type rule struct {
	pkg       string
	forbidden []string
	why       string
}

var rules = []rule{
	{"core", []string{"quota", "nbtier", "nbopinion", "catalog", "telemetry", "adapt", "policyfile", "adapters"},
		"core production sources reach those packages only through hooks registered by cmd"},
	{"session", []string{"core"}, "session is a leaf loader; PlanForSession and friends stay in core"},
	{"fuse", []string{"core"}, "fuse imports classify and semantic, never core (core wraps fuse)"},
	{"codeintel", []string{"core", "classify"}, "code intelligence is independent of routing"},
	{"telemetry", []string{"accounting", "api", "core"}, "L0: events are parsed here; the numbers are computed in accounting. Router values arrive through telemetry/fromcore, the only telemetry package that imports core (TestTelemetryTreeImportsCoreOnlyThroughFromcore checks the rest of the tree)"},
	{"execwrap", []string{"accounting", "api"}, "L0: the exec log is parsed here; the numbers are computed in accounting"},
	{"accounting", []string{"api", "adapters"}, "L3: Compute serves the surfaces, so it cannot depend on them"},
	{"accounting/formula", []string{"telemetry", "execwrap", "api", "adapters"}, "L2: the formula is pure and reads only prices"},
}

// violations returns the entries of deps (import paths) that match a forbidden
// package. Pure, so the negative test can prove it detects a violation.
func violations(deps, forbidden []string) []string {
	var out []string
	for _, d := range deps {
		if !strings.HasPrefix(d, module) {
			continue
		}
		rel := strings.TrimPrefix(d, module)
		for _, f := range forbidden {
			if rel == f || strings.HasPrefix(rel, f+"/") {
				out = append(out, rel)
				break
			}
		}
	}
	sort.Strings(out)
	return out
}

func TestImportGraph(t *testing.T) {
	for _, r := range rules {
		t.Run(r.pkg, func(t *testing.T) {
			// -deps without -test lists production imports only.
			out, err := exec.Command("go", "list", "-deps", module+r.pkg).Output()
			if err != nil {
				t.Fatalf("go list -deps %s: %v", r.pkg, err)
			}
			deps := strings.Fields(string(out))
			if len(deps) == 0 {
				t.Fatalf("no dependencies listed for %s", r.pkg)
			}
			if v := violations(deps, r.forbidden); len(v) > 0 {
				t.Errorf("internal/%s must not depend on %v (%s)", r.pkg, v, r.why)
			}
		})
	}
}

func TestViolationsDetectsForbiddenImports(t *testing.T) {
	deps := []string{
		"fmt",
		module + "core",
		module + "classify",
		module + "quotamodel", // allowed: pure types, not quota
		module + "telemetry",
		module + "adapters/claudecode",
		"github.com/other/telemetry", // outside the module: ignored
	}
	got := violations(deps, []string{"quota", "telemetry", "adapters"})
	want := []string{"adapters/claudecode", "telemetry"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("violations = %v, want %v", got, want)
	}
	if v := violations(deps, []string{"nbtier"}); len(v) != 0 {
		t.Fatalf("false positive: %v", v)
	}
}

// The rules table checks the telemetry root. This checks every other package
// under telemetry, so a new one cannot import core unnoticed. fromcore is the
// composition point that installs the router's values, and the only exception.
func TestTelemetryTreeImportsCoreOnlyThroughFromcore(t *testing.T) {
	out, err := exec.Command("go", "list", "-f", "{{.ImportPath}} {{join .Deps \" \"}}", module+"telemetry/...").Output()
	if err != nil {
		t.Fatalf("go list telemetry tree: %v", err)
	}
	checked := 0
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		pkg := strings.TrimPrefix(fields[0], module)
		if pkg == "telemetry/fromcore" {
			continue
		}
		checked++
		if v := violations(fields[1:], []string{"core"}); len(v) > 0 {
			t.Errorf("internal/%s must not depend on core (%v): only telemetry/fromcore may", pkg, v)
		}
	}
	if checked == 0 {
		t.Fatal("go list found no telemetry packages to check")
	}
}

// Telemetry reads the router's values from DirectionCatalog and
// DeclaredHarnesses, which only fromcore.Install sets. A command that imports
// telemetry and never installs them gets empty reports and no error. This test
// reads the command sources and asks for a call to fromcore.Install. It sees
// the wiring, not the order of the calls.
func TestEntryPointsInstallTheRouter(t *testing.T) {
	checked, missing, err := commandsWithoutInstall(filepath.Join("..", "..", "cmd"))
	if err != nil {
		t.Fatal(err)
	}
	if checked == 0 {
		t.Fatal("no command under cmd/ imports telemetry")
	}
	for _, dir := range missing {
		t.Errorf("%s imports telemetry but never calls fromcore.Install", dir)
	}
}

func TestCommandsWithoutInstallDetectsMissingCall(t *testing.T) {
	root := t.TempDir()
	write := func(name, src string) {
		dir := filepath.Join(root, name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("good", `package main

import (
	"github.com/tiagovilasboas/downshift/internal/telemetry"
	"github.com/tiagovilasboas/downshift/internal/telemetry/fromcore"
)

func main() { fromcore.Install(nil); _ = telemetry.DeclaredHarnesses }
`)
	write("bad", `package main

import "github.com/tiagovilasboas/downshift/internal/telemetry"

func main() { _ = telemetry.DeclaredHarnesses }
`)
	write("unrelated", `package main

func main() {}
`)
	checked, missing, err := commandsWithoutInstall(root)
	if err != nil {
		t.Fatal(err)
	}
	if checked != 2 {
		t.Fatalf("checked %d commands, want the two that import telemetry", checked)
	}
	if len(missing) != 1 || filepath.Base(missing[0]) != "bad" {
		t.Fatalf("missing = %v, want only bad", missing)
	}
}

// commandsWithoutInstall lists the command directories under root that import
// telemetry and have no fromcore.Install call in their non-test sources. It
// returns how many commands import telemetry at all.
func commandsWithoutInstall(root string) (checked int, missing []string, err error) {
	dirs, err := filepath.Glob(filepath.Join(root, "*"))
	if err != nil {
		return 0, nil, err
	}
	for _, dir := range dirs {
		imports, installs, err := scanCommand(dir)
		if err != nil {
			return 0, nil, err
		}
		if !slices.ContainsFunc(imports, func(p string) bool { return strings.HasPrefix(p, module+"telemetry") }) {
			continue
		}
		checked++
		if !installs {
			missing = append(missing, dir)
		}
	}
	return checked, missing, nil
}

// scanCommand parses the non-test Go files of one directory. It returns their
// import paths and whether one of them calls fromcore.Install.
func scanCommand(dir string) (imports []string, installs bool, err error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		return nil, false, err
	}
	fset := token.NewFileSet()
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			return nil, false, err
		}
		for _, imp := range f.Imports {
			imports = append(imports, strings.Trim(imp.Path.Value, `"`))
		}
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "Install" {
				return true
			}
			if id, ok := sel.X.(*ast.Ident); ok && id.Name == "fromcore" {
				installs = true
			}
			return true
		})
	}
	return imports, installs, nil
}
