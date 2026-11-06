// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor_test

// Source readers for the fixture coverage report: the compressor Format
// constants, the execwrap Family constants and execwrap's expectedFormats map.
// They parse the Go sources, so a new Format or Family shows up in the report
// without anyone editing a list.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

const execwrapDir = "../execwrap"

// stringConsts returns name -> value for every const of the named type with a
// string literal value, across the non-test files matching glob.
func stringConsts(t *testing.T, glob, typeName string) map[string]string {
	t.Helper()
	files, err := filepath.Glob(filepath.FromSlash(glob))
	if err != nil || len(files) == 0 {
		t.Fatalf("no source files for %s: %v", glob, err)
	}
	out := map[string]string{}
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range file.Decls {
			gd, ok := d.(*ast.GenDecl)
			if !ok || gd.Tok != token.CONST {
				continue
			}
			for _, spec := range gd.Specs {
				vs := spec.(*ast.ValueSpec)
				id, ok := vs.Type.(*ast.Ident)
				if !ok || id.Name != typeName || len(vs.Values) != 1 {
					continue
				}
				lit, ok := vs.Values[0].(*ast.BasicLit)
				if !ok {
					continue
				}
				if val, err := strconv.Unquote(lit.Value); err == nil && val != "" {
					out[vs.Names[0].Name] = val
				}
			}
		}
	}
	return out
}

// formatConsts maps the Go name of each compressor Format constant to its value.
func formatConsts(t *testing.T) map[string]string {
	t.Helper()
	out := stringConsts(t, "*.go", "Format")
	if len(out) == 0 {
		t.Fatal("no Format constants found in internal/compressor")
	}
	return out
}

// familyConsts maps the Go name of each execwrap Family constant to its value.
func familyConsts(t *testing.T) map[string]string {
	t.Helper()
	out := stringConsts(t, execwrapDir+"/policy*.go", "Family")
	if len(out) == 0 {
		t.Fatal("no Family constants found in internal/execwrap")
	}
	return out
}

// expectedFormatsOf maps each execwrap family value to the format values its
// output may be compacted as (execwrap/expected_format.go, expectedFormats).
// A family with no entry passes through.
func expectedFormatsOf(t *testing.T) map[string][]string {
	t.Helper()
	families, formats := familyConsts(t), formatConsts(t)
	path := filepath.FromSlash(execwrapDir + "/expected_format.go")
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string][]string{}
	ast.Inspect(file, func(n ast.Node) bool {
		vs, ok := n.(*ast.ValueSpec)
		if !ok || len(vs.Names) != 1 || vs.Names[0].Name != "expectedFormats" || len(vs.Values) != 1 {
			return true
		}
		lit, ok := vs.Values[0].(*ast.CompositeLit)
		if !ok {
			return true
		}
		for _, el := range lit.Elts {
			kv := el.(*ast.KeyValueExpr)
			fam := families[kv.Key.(*ast.Ident).Name]
			list, ok := kv.Value.(*ast.CompositeLit)
			if !ok || fam == "" {
				t.Fatalf("expectedFormats entry %v is not Family: {formats}", kv.Key)
			}
			for _, f := range list.Elts {
				name := f.(*ast.SelectorExpr).Sel.Name
				if formats[name] == "" {
					t.Fatalf("expectedFormats names unknown format %s", name)
				}
				out[fam] = append(out[fam], formats[name])
			}
		}
		return false
	})
	if len(out) == 0 {
		t.Fatal("expectedFormats not found in execwrap/expected_format.go")
	}
	return out
}
