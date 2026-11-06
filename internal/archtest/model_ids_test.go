// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package archtest

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// modelIDBaseline freezes the model id literals that production code outside
// internal/catalog carried when this sensor was added. Each entry is a file
// and the exact literal. The list may only shrink: an entry that no longer
// violates fails the test, so delete it when you remove the literal.
var modelIDBaseline = map[string][]string{}

// catalogModelPrefixes derives the model id prefixes ("claude-haiku",
// "gpt-5", "muse-spark-1.3", ...) from the embedded catalog: every id, alias
// and family. A literal starting with one is a hardcoded model id. The sensor
// reads the catalog rather than hardcoding the list.
func catalogModelPrefixes(t *testing.T) []string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(repoRoot, "internal", "catalog", "catalog.json"))
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Entries []struct {
			ID      string   `json:"id"`
			Family  string   `json:"family"`
			Aliases []string `json:"aliases"`
		} `json:"entries"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, e := range doc.Entries {
		for _, s := range append([]string{e.ID, e.Family}, e.Aliases...) {
			// Plain words ("pro", "sol", "haiku") would match ordinary
			// text; only dashed or numbered names are model-id shaped.
			if strings.ContainsAny(s, "-0123456789") {
				seen[s] = true
			}
		}
	}
	var out []string
	for p := range seen {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

// modelIDLiterals returns the string literals of a Go file that start with one
// of the catalog prefixes.
func modelIDLiterals(t *testing.T, path string, prefixes []string) []string {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), filepath.Join(repoRoot, filepath.FromSlash(path)), nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	ast.Inspect(f, func(n ast.Node) bool {
		lit, ok := n.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		s, err := strconv.Unquote(lit.Value)
		if err != nil {
			return true
		}
		for _, p := range prefixes {
			if strings.HasPrefix(s, p) {
				out = append(out, s)
				break
			}
		}
		return true
	})
	return out
}

// TestNoModelIDLiterals is the computational sensor for "do not add model IDs
// as hardcoded strings in Go source; use catalog.json" (AGENTS.md). Test files
// and internal/catalog are exempt.
func TestNoModelIDLiterals(t *testing.T) {
	prefixes := catalogModelPrefixes(t)
	if len(prefixes) == 0 {
		t.Fatal("no model id prefixes found in internal/catalog/catalog.json")
	}
	found := map[string][]string{}
	for _, p := range allProductionGoFiles(t) {
		if strings.HasPrefix(p, "internal/catalog/") {
			continue
		}
		if lits := modelIDLiterals(t, p, prefixes); len(lits) > 0 {
			found[p] = lits
		}
	}
	for file, lits := range found {
		allowed := map[string]bool{}
		for _, a := range modelIDBaseline[file] {
			allowed[a] = true
		}
		for _, lit := range lits {
			if !allowed[lit] {
				t.Errorf("%s: model id literal %q. Rule: model ids live in internal/catalog/catalog.json, not in Go source. Fix: resolve the model through the catalog (core.Resolver) instead of writing the id; do not add it to modelIDBaseline.", file, lit)
			}
		}
	}
	for file, lits := range modelIDBaseline {
		for _, want := range lits {
			if !slices.Contains(found[file], want) {
				t.Errorf("modelIDBaseline entry %s %q no longer violates: remove it from the baseline", file, want)
			}
		}
	}
}
