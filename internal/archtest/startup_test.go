// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package archtest

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// Startup guard (roadmap H3). Every hook call and every wrapped command starts
// the binary, and every package linked into it runs its init on every start. A
// package-level regexp.MustCompile compiles at init whether the command uses it
// or not: 450 of them made init ~14 ms of a ~19 ms start. Declare the
// expression with lazyre.New (compiled on first use) or build it inside a
// sync.OnceValue instead.

// regexpCompileFuncs are the regexp functions that compile an expression.
var regexpCompileFuncs = map[string]bool{
	"Compile": true, "MustCompile": true, "CompilePOSIX": true, "MustCompilePOSIX": true,
}

// eagerRegexpAllowlist names production files that may still compile a
// regular expression at init, with the count allowed. It is empty: use
// lazyre.New. Do not add an entry without measuring the start time.
var eagerRegexpAllowlist = map[string]int{}

// eagerRegexps returns the positions of regexp compile calls that run at
// package init in src: in a package-level var initializer (outside a function
// literal, which runs later) or in a func init body.
func eagerRegexps(filename string, src any) ([]string, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filename, src, 0)
	if err != nil {
		return nil, err
	}
	name := regexpImportName(f)
	if name == "" {
		return nil, nil
	}
	var out []string
	visit := func(root ast.Node) {
		ast.Inspect(root, func(n ast.Node) bool {
			if _, ok := n.(*ast.FuncLit); ok {
				return false
			}
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if id, ok := sel.X.(*ast.Ident); ok && id.Name == name && regexpCompileFuncs[sel.Sel.Name] {
				out = append(out, fmt.Sprintf("%s: regexp.%s", fset.Position(call.Pos()), sel.Sel.Name))
			}
			return true
		})
	}
	for _, d := range f.Decls {
		switch d := d.(type) {
		case *ast.GenDecl:
			if d.Tok == token.VAR {
				visit(d)
			}
		case *ast.FuncDecl:
			if d.Recv == nil && d.Name.Name == "init" && d.Body != nil {
				visit(d.Body)
			}
		}
	}
	return out, nil
}

// regexpImportName is the local name of the "regexp" import, or "".
func regexpImportName(f *ast.File) string {
	for _, imp := range f.Imports {
		p, err := strconv.Unquote(imp.Path.Value)
		if err != nil || p != "regexp" {
			continue
		}
		if imp.Name != nil {
			return imp.Name.Name
		}
		return path.Base(p)
	}
	return ""
}

// TestNoRegexpCompiledAtInit fails on a package-level regexp compile in any
// production file: the binary links nearly every package, so each one costs
// every start.
func TestNoRegexpCompiledAtInit(t *testing.T) {
	for _, p := range allProductionGoFiles(t) {
		hits, err := eagerRegexps(filepath.Join(repoRoot, filepath.FromSlash(p)), nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(hits) > eagerRegexpAllowlist[p] {
			t.Errorf("%s compiles %d regular expression(s) at package init:\n  %s\nRule: no regexp compile at init (every hook call starts the binary). Fix: declare it with lazyre.New(...) from internal/lazyre, or build it inside sync.OnceValue.",
				p, len(hits), strings.Join(hits, "\n  "))
		}
	}
}

// TestLazyRegexpPackagesCompileThemInTests: an invalid lazy expression panics
// on first use, not at init, so a package that declares any must call
// lazyre.CompileAll from a test.
func TestLazyRegexpPackagesCompileThemInTests(t *testing.T) {
	const lazyImport = `"` + modulePath + `/internal/lazyre"`
	declares := map[string]bool{}
	for _, p := range allProductionGoFiles(t) {
		if strings.HasPrefix(p, "internal/lazyre/") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(p)))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(raw), lazyImport) {
			declares[path.Dir(p)] = true
		}
	}
	compiles := map[string]bool{}
	for _, p := range walkFiles(t, "_test.go") {
		raw, err := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(p)))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(raw), "lazyre.CompileAll()") {
			compiles[path.Dir(p)] = true
		}
	}
	if len(declares) == 0 {
		t.Fatal("no package imports internal/lazyre; the sensor is not looking at the tree")
	}
	for dir := range declares {
		if !compiles[dir] {
			t.Errorf("%s declares lazyre expressions but no test calls lazyre.CompileAll(). Fix: add a lazyre_test.go like internal/compressor/lazyre_test.go.", dir)
		}
	}
}

func TestEagerRegexpsDetectsInitCompiles(t *testing.T) {
	src := `package p

import re "regexp"

var a = re.MustCompile("a")

var (
	b = []*re.Regexp{re.MustCompile("b")}
	lazy = func() *re.Regexp { return re.MustCompile("c") }
)

func init() { _ = re.MustCompile("d") }

func f() { _ = re.MustCompile("e") }
`
	hits, err := eagerRegexps("p.go", src)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 3 {
		t.Fatalf("eagerRegexps found %d, want 3 (a, b, init): %v", len(hits), hits)
	}
	none, err := eagerRegexps("q.go", "package q\n\nvar x = 1\n")
	if err != nil || len(none) != 0 {
		t.Fatalf("file without regexp: %v %v", none, err)
	}
}
