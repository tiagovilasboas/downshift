// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"strings"
	"testing"
)

// A change that touches only Markdown still runs the docscheck guards, and
// nothing else: README.md at the root and a file under docs/ alike.
func TestMarkdownOnlyChangeSelectsDocscheck(t *testing.T) {
	t.Chdir("../..")
	for _, files := range [][]string{{"README.md"}, {"docs/reference/cli.md"}, {"CHANGELOG.md", "docs/guides/config.md"}} {
		pkgs, err := selectPackages(files)
		if err != nil {
			t.Fatal(err)
		}
		if len(pkgs) != 1 || !strings.HasSuffix(pkgs[0], "/internal/docscheck") {
			t.Errorf("selectPackages(%v) = %v, want only internal/docscheck", files, pkgs)
		}
	}
}
