// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/lazyre"
)

// TestLazyRegexpsCompile compiles every package-level expression, so an
// invalid one fails here rather than on the first run that uses it.
func TestLazyRegexpsCompile(t *testing.T) {
	if lazyre.CompileAll() == 0 {
		t.Fatal("no lazy expressions registered")
	}
}
