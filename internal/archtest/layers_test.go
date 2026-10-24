// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package archtest

import (
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// repoRoot is the module root; the test runs in internal/archtest.
const repoRoot = "../.."

// directImports lists the module packages (relative to internal/) that the
// production files of pkg import directly. -f .Imports excludes test files.
func directImports(t *testing.T, pkg string) []string {
	t.Helper()
	out, err := exec.Command("go", "list", "-f", `{{join .Imports "\n"}}`, module+pkg).Output()
	if err != nil {
		t.Fatalf("go list %s: %v", pkg, err)
	}
	var rel []string
	for _, imp := range strings.Fields(string(out)) {
		if strings.HasPrefix(imp, module) {
			rel = append(rel, strings.TrimPrefix(imp, module))
		}
	}
	sort.Strings(rel)
	return rel
}

// outsideAllowlist returns the imports not in allowed. Pure, so the negative
// test can prove it detects a violation.
func outsideAllowlist(imports, allowed []string) []string {
	ok := map[string]bool{}
	for _, a := range allowed {
		ok[a] = true
	}
	var bad []string
	for _, i := range imports {
		if !ok[i] {
			bad = append(bad, i)
		}
	}
	return bad
}

// TestLayerDirectImports: L3 (accounting) and L2 (formula) import only the
// layers below them, directly.
func TestLayerDirectImports(t *testing.T) {
	allowed := map[string][]string{
		"accounting":         {"telemetry", "execwrap", "catalog", "accounting/formula"},
		"accounting/formula": {"catalog"},
	}
	for pkg, ok := range allowed {
		t.Run(pkg, func(t *testing.T) {
			if bad := outsideAllowlist(directImports(t, pkg), ok); len(bad) > 0 {
				t.Errorf("internal/%s imports %v; it may import only %v", pkg, bad, ok)
			}
		})
	}
}

func TestOutsideAllowlistDetectsExtraImports(t *testing.T) {
	got := outsideAllowlist([]string{"telemetry", "api", "catalog"}, []string{"telemetry", "catalog"})
	if len(got) != 1 || got[0] != "api" {
		t.Fatalf("outsideAllowlist = %v, want [api]", got)
	}
}

// importsOf parses only the import block of a file.
func importsOf(t *testing.T, path string) []string {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	var out []string
	for _, imp := range f.Imports {
		out = append(out, strings.Trim(imp.Path.Value, `"`))
	}
	return out
}

// productionGoFiles lists the non-test .go files directly in dir (repo-relative).
func productionGoFiles(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(repoRoot, dir))
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range entries {
		if n := e.Name(); !e.IsDir() && strings.HasSuffix(n, ".go") && !strings.HasSuffix(n, "_test.go") {
			out = append(out, dir+"/"+n)
		}
	}
	return out
}

// TestSurfacesImportAccounting: the surfaces render accounting.Numbers, so the
// API package and the report and panel commands must import accounting.
func TestSurfacesImportAccounting(t *testing.T) {
	const accounting = module + "accounting"
	has := func(files ...string) bool {
		for _, f := range files {
			for _, imp := range importsOf(t, filepath.Join(repoRoot, f)) {
				if imp == accounting {
					return true
				}
			}
		}
		return false
	}
	if !has(productionGoFiles(t, "internal/api")...) {
		t.Errorf("internal/api no longer imports internal/accounting: its figures must come from accounting.Compute")
	}
	for _, f := range []string{"cmd/downshift/report.go", "cmd/downshift/sessionpanel.go"} {
		if !has(f) {
			t.Errorf("%s does not import internal/accounting: it must render accounting.Numbers", f)
		}
	}
}
