// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"slices"
	"strings"
	"testing"
)

// A changed .go file always selects the whole-tree guards, whatever package it is in.
func TestSelectPackagesAlwaysIncludesGuardsForGoChanges(t *testing.T) {
	t.Chdir("../..")
	pkgs, err := selectPackages([]string{"internal/testenv/timing.go"})
	if err != nil {
		t.Fatal(err)
	}
	for _, guard := range []string{"internal/archtest", "internal/docscheck", "internal/limits"} {
		if !slices.ContainsFunc(pkgs, func(p string) bool { return strings.HasSuffix(p, "/"+guard) }) {
			t.Errorf("%s not selected for a .go change; got %v", guard, pkgs)
		}
	}
}
