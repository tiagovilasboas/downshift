// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package gateway_test

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// sharedPackages may import only the gateway port, never an adapter.
var sharedPackages = []string{"core", "telemetry"}

// TestSharedLayersImportOnlyThePort fails when core or telemetry imports a
// gateway adapter. Only the port (internal/gateway) is allowed.
func TestSharedLayersImportOnlyThePort(t *testing.T) {
	for _, pkg := range sharedPackages {
		dir := filepath.Join("..", pkg)
		files, err := goFiles(dir)
		if err != nil {
			t.Fatal(err)
		}
		for _, file := range files {
			f, err := parser.ParseFile(token.NewFileSet(), file, nil, parser.ImportsOnly)
			if err != nil {
				t.Fatalf("parse %s: %v", file, err)
			}
			for _, imp := range f.Imports {
				path := strings.Trim(imp.Path.Value, `"`)
				if strings.Contains(path, "/internal/gateway/") {
					t.Errorf("%s imports adapter %s; shared layers import only internal/gateway", file, path)
				}
			}
		}
	}
}

// TestCoreNamesNoRouter fails when a core production source names a router.
// Router knowledge lives in an adapter; core speaks in gateway terms.
func TestCoreNamesNoRouter(t *testing.T) {
	files, err := goFiles(filepath.Join("..", "core"))
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(strings.ToLower(string(data)), "openrouter") {
			t.Errorf("%s names a router; use the gateway port wording", file)
		}
	}
}

// goFiles lists the non-test Go sources in dir.
func goFiles(dir string) ([]string, error) {
	matches, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		return nil, err
	}
	var out []string
	for _, m := range matches {
		if !strings.HasSuffix(m, "_test.go") {
			out = append(out, m)
		}
	}
	return out, nil
}
