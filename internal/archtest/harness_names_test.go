// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package archtest

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// harnessNames are the harness identifiers shared code must not branch on.
var harnessNames = []string{"cursor", "codex", "opencode", "claude-code", "claudecode", "antigravity", "kirocrew"}

// harnessSharedTrees are the shared packages (path prefixes) where harness
// facts must be data in HarnessCapabilities, never a branch.
var harnessSharedTrees = []string{
	"internal/core/", "internal/routing/", "internal/telemetry/", "internal/policyfile/",
	"internal/quota/", "internal/execwrap/", "internal/shellwrap/", "internal/postrun/",
}

// harnessBranchBaseline freezes the harness-name branches that shared packages
// carried when this sensor was added: file -> literals compared. The list may
// only shrink: an entry that no longer violates fails the test.
var harnessBranchBaseline = map[string][]string{
	// The Codex quota source reads Codex transcripts and snapshots by design.
	"internal/quota/codex_transcript.go": {"codex"},
	"internal/quota/native.go":           {"codex"},
}

func isHarnessLiteral(e ast.Expr) (string, bool) {
	lit, ok := e.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	s, err := strconv.Unquote(lit.Value)
	if err != nil || !slices.Contains(harnessNames, s) {
		return "", false
	}
	return s, true
}

// harnessBranches returns the harness-name literals a file compares in an
// if condition, a == / != expression, or a switch case.
func harnessBranches(t *testing.T, path string) []string {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), filepath.Join(repoRoot, filepath.FromSlash(path)), nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	set := map[string]bool{}
	add := func(e ast.Expr) {
		if s, ok := isHarnessLiteral(e); ok {
			set[s] = true
		}
	}
	ast.Inspect(f, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.BinaryExpr:
			if x.Op == token.EQL || x.Op == token.NEQ {
				add(x.X)
				add(x.Y)
			}
		case *ast.CaseClause:
			for _, e := range x.List {
				add(e)
			}
		}
		return true
	})
	var out []string
	for s := range set {
		out = append(out, s)
	}
	slices.Sort(out)
	return out
}

// TestNoHarnessNameBranches is the computational sensor for layering rule 3
// (AGENTS.md): harness facts are data in HarnessCapabilities, never branches
// in shared code. Adapters are not in scope.
func TestNoHarnessNameBranches(t *testing.T) {
	found := map[string][]string{}
	for _, p := range allProductionGoFiles(t) {
		shared := false
		for _, tree := range harnessSharedTrees {
			shared = shared || strings.HasPrefix(p, tree)
		}
		if !shared {
			continue
		}
		if names := harnessBranches(t, p); len(names) > 0 {
			found[p] = names
		}
	}
	for file, names := range found {
		for _, name := range names {
			if !slices.Contains(harnessBranchBaseline[file], name) {
				t.Errorf("%s: branches on the harness name %q. Rule (layering rule 3): harness facts are data in HarnessCapabilities, never branches in shared code. Fix: add a capability field read via core.AllHarnessCaps() and branch on that; do not add to harnessBranchBaseline.", file, name)
			}
		}
	}
	for file, names := range harnessBranchBaseline {
		for _, name := range names {
			if !slices.Contains(found[file], name) {
				t.Errorf("harnessBranchBaseline entry %s %q no longer violates: remove it from the baseline", file, name)
			}
		}
	}
}
