// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/clispec"
)

// dispatchedNames reads main.go's switch on args[0] and the hook table, so the
// test follows the code instead of a second list.
func dispatchedNames(t *testing.T) []string {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), "main.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	ast.Inspect(file, func(n ast.Node) bool {
		cc, ok := n.(*ast.CaseClause)
		if !ok {
			return true
		}
		for _, e := range cc.List {
			if lit, ok := e.(*ast.BasicLit); ok && lit.Kind == token.STRING {
				s, _ := strconv.Unquote(lit.Value)
				names = append(names, s)
			}
		}
		return true
	})
	for id, h := range harnessRegistry {
		if h.Hook != nil {
			names = append(names, id)
		}
	}
	// main.go also handles these before the switch.
	return append(names, "compact", "compactor", "exec", "retrieve")
}

func TestEveryDispatchedCommandIsInTheRegistry(t *testing.T) {
	names := dispatchedNames(t)
	if len(names) < 30 {
		t.Fatalf("parsed only %d dispatched names: %v", len(names), names)
	}
	for _, n := range names {
		if _, ok := clispec.Lookup(n); !ok {
			t.Errorf("dispatched command %q is not in internal/clispec", n)
		}
	}
}

func TestEveryTopLevelHelpTopicIsInTheRegistry(t *testing.T) {
	for topic := range helpTopics {
		if _, ok := clispec.Lookup(topic); !ok {
			t.Errorf("help topic %q is not in internal/clispec", topic)
		}
	}
}

func TestEveryRegistryCommandIsDispatched(t *testing.T) {
	dispatched := map[string]bool{}
	for _, n := range dispatchedNames(t) {
		dispatched[n] = true
	}
	for _, c := range clispec.Commands() {
		if containsSpace(c.Name) || dispatched[c.Name] {
			continue
		}
		t.Errorf("registry command %q is not dispatched by main.go", c.Name)
	}
}

func containsSpace(s string) bool {
	for _, r := range s {
		if r == ' ' {
			return true
		}
	}
	return false
}
