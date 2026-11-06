// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package archtest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	headerCopyright = "// Copyright (c) 2026 Tiago Vilas Boas."
	headerSPDX      = "// SPDX-License-Identifier: Apache-2.0"
)

// hasHeader reports whether the file carries the two header lines, allowing
// build constraints (and blank lines) before them: a build line must come
// first in the file.
func hasHeader(src string) bool {
	lines := strings.Split(src, "\n")
	i := 0
	for i < len(lines) {
		l := lines[i]
		if strings.HasPrefix(l, "//go:build") || strings.HasPrefix(l, "// +build") || strings.TrimSpace(l) == "" {
			i++
			continue
		}
		break
	}
	// A CRLF checkout (Windows autocrlf) leaves a trailing \r on each line.
	trim := func(l string) string { return strings.TrimRight(l, "\r") }
	return i+1 < len(lines) && trim(lines[i]) == headerCopyright && trim(lines[i+1]) == headerSPDX
}

// isGenerated reports whether the file carries the standard generated marker
// before its package clause.
func isGenerated(src string) bool {
	for _, l := range strings.Split(src, "\n") {
		if strings.HasPrefix(l, "// Code generated") && strings.HasSuffix(l, "DO NOT EDIT.") {
			return true
		}
		if strings.HasPrefix(l, "package ") {
			return false
		}
	}
	return false
}

// TestSPDXHeader is the computational sensor for the AGENTS.md copyright
// header rule: every non-generated Go file, production or test, starts with
// the copyright and SPDX lines. Exempt: internal/outcome/testdata (stub.go is
// pasted into a model prompt, so a header would change what the suite
// measures) and generated files.
func TestSPDXHeader(t *testing.T) {
	for _, p := range walkFiles(t, ".go") {
		if strings.HasPrefix(p, "internal/outcome/testdata/") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(p)))
		if err != nil {
			t.Fatal(err)
		}
		src := string(raw)
		if isGenerated(src) || hasHeader(src) {
			continue
		}
		t.Errorf("%s: missing the licence header. Rule (AGENTS.md, Copyright headers): every Go file starts with\n%s\n%s\nFix: add those two lines at the top of the file (after any build-constraint line).", p, headerCopyright, headerSPDX)
	}
}
