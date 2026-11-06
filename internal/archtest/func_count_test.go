// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package archtest

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"testing"
)

// maxFuncsPerFile is the common-gate limit of top-level functions and methods
// in a production Go file.
const maxFuncsPerFile = 10

// funcCountBaseline freezes the files above the limit when this sensor was
// added, each with its count at that time as a ceiling: a file may go down,
// never up. A file that falls to the limit or below must be removed.
var funcCountBaseline = map[string]int{
	"cmd/downshift/apply_core.go":                        12,
	"cmd/downshift/doctor_checks.go":                     11,
	"cmd/downshift/doctor_plugin_hooks.go":               11,
	"internal/adapters/claudecode/event.go":              11,
	"internal/adapters/claudecode/posttooluse.go":        12,
	"internal/adapters/codex/codex.go":                   13,
	"internal/adapters/kirocrew/kirocrew.go":             12,
	"internal/catalog/catalog_entries.go":                14,
	"internal/console/app.go":                            13,
	"internal/console/reportview/model.go":               14,
	"internal/console/shared/diffview/diffview.go":       11,
	"internal/contextopt/providers/headroom/headroom.go": 15,
	"internal/contextopt/providers/native/native.go":     14,
	"internal/core/session_resolver.go":                  13,
	"internal/discovery/sources.go":                      13,
	"internal/paths/paths.go":                            11,
	"internal/telemetry/aggregate.go":                    13,
	"internal/telemetry/panel.go":                        12,
	"internal/telemetry/routing_tokens.go":               11,
	"internal/telemetry/spawn_state.go":                  14,
	"internal/workers/write.go":                          14,
}

func topLevelFuncs(t *testing.T, path string) int {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), filepath.Join(repoRoot, filepath.FromSlash(path)), nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, d := range f.Decls {
		if _, ok := d.(*ast.FuncDecl); ok {
			n++
		}
	}
	return n
}

// TestProductionFuncCount is the computational sensor for "about 10 functions
// per file": split by what the code does, not by type.
func TestProductionFuncCount(t *testing.T) {
	counts := map[string]int{}
	for _, p := range allProductionGoFiles(t) {
		counts[p] = topLevelFuncs(t, p)
	}
	for path, n := range counts {
		ceiling, frozen := funcCountBaseline[path]
		switch {
		case frozen && n > ceiling:
			t.Errorf("%s has %d functions, above its frozen ceiling %d. Rule: at most %d functions per file. Fix: move related functions to a new file named for their domain.", path, n, ceiling, maxFuncsPerFile)
		case frozen && n <= maxFuncsPerFile:
			t.Errorf("%s is now %d functions, within the limit: remove its funcCountBaseline entry", path, n)
		case frozen && n < ceiling:
			t.Errorf("%s shrank to %d functions: lower its funcCountBaseline entry from %d", path, n, ceiling)
		case !frozen && n > maxFuncsPerFile:
			t.Errorf("%s has %d functions, over the %d-function limit. Rule: split files by responsibility. Fix: move related functions to a new file named for their domain; do not add to funcCountBaseline.", path, n, maxFuncsPerFile)
		}
	}
	for path, ceiling := range funcCountBaseline {
		if _, ok := counts[path]; !ok {
			t.Errorf("funcCountBaseline entry %s (%d) names a file that no longer exists: remove it", path, ceiling)
		}
	}
}
