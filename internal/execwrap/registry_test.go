// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

// Registry check: a new family is complete only when every shared list names
// it. Family constants are read from policy.go, so adding one without an
// expectedFormat entry or corpus fixtures fails here with the missing piece.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

const corpusManifest = "../compressor/testdata/corpus/MANIFEST"

// familyConstants returns the name and value of every Family constant declared
// in policy.go, except FamilyNone.
func familyConstants(t *testing.T) map[string]Family {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), "policy.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]Family{}
	for _, d := range file.Decls {
		gd, ok := d.(*ast.GenDecl)
		if !ok || gd.Tok != token.CONST {
			continue
		}
		for _, spec := range gd.Specs {
			vs := spec.(*ast.ValueSpec)
			id, ok := vs.Type.(*ast.Ident)
			if !ok || id.Name != "Family" || len(vs.Values) != 1 {
				continue
			}
			lit, ok := vs.Values[0].(*ast.BasicLit)
			if !ok {
				continue
			}
			val, err := strconv.Unquote(lit.Value)
			if err != nil || val == "" {
				continue
			}
			out[vs.Names[0].Name] = Family(val)
		}
	}
	if len(out) == 0 {
		t.Fatal("no Family constants found in policy.go")
	}
	return out
}

// manifestKinds counts the clean and preserve fixtures per format.
func manifestKinds(t *testing.T) map[string]map[string]int {
	t.Helper()
	raw, err := os.ReadFile(filepath.FromSlash(corpusManifest))
	if err != nil {
		t.Fatalf("corpus manifest missing: %v", err)
	}
	out := map[string]map[string]int{}
	for _, line := range strings.Split(string(raw), "\n") {
		f := strings.Fields(line)
		if len(f) != 3 || strings.HasPrefix(f[0], "#") {
			continue
		}
		if out[f[1]] == nil {
			out[f[1]] = map[string]int{}
		}
		out[f[1]][f[0]]++
	}
	return out
}

func TestRegistryEveryFamilyHasFormatAndCorpus(t *testing.T) {
	kinds := manifestKinds(t)
	for name, fam := range familyConstants(t) {
		format, ok := expectedFormat[fam]
		if !ok {
			t.Errorf("%s (%q) has no expectedFormat entry in run.go", name, fam)
			continue
		}
		for _, kind := range []string{"clean", "preserve"} {
			if kinds[string(format)][kind] == 0 {
				t.Errorf("%s: format %q has no %s fixture in compressor/testdata/corpus/MANIFEST", name, format, kind)
			}
		}
	}
}

func TestRegistryExpectedFormatNamesAFamily(t *testing.T) {
	known := map[Family]bool{}
	for _, fam := range familyConstants(t) {
		known[fam] = true
	}
	for fam := range expectedFormat {
		if !known[fam] {
			t.Errorf("expectedFormat has an entry for %q, which is not a Family constant in policy.go", fam)
		}
	}
}
