// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package docscheck_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// historyMarker excuses the line it sits on, or, when it is alone on its line,
// every line after it up to the next blank line outside a code fence (a table
// row, a paragraph or a whole code block), from the drift guards: the text records what was true then.
const historyMarker = "<!-- docscheck:history -->"

// docLine is one line of a guarded Markdown file.
type docLine struct {
	File string // slash-separated, relative to the root
	N    int
	Text string
}

// historyDirs hold dated snapshots (audits, spikes, release notes, reviews):
// they record what was true then, so the drift guards skip them.
var historyDirs = map[string]bool{
	"docs/internal": true, "docs/archive": true, "docs/evidence": true,
	"docs/spikes": true, "docs/releases": true, "docs/product": true,
}

// guardedDocs are the Markdown files the drift guards read: README.md,
// CHANGELOG.md and docs/**, minus the historyDirs.
func guardedDocs(t *testing.T, root string) []string {
	t.Helper()
	files := []string{"README.md", "CHANGELOG.md"}
	err := filepath.WalkDir(filepath.Join(root, "docs"), func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		rel = filepath.ToSlash(rel)
		if d.IsDir() && historyDirs[rel] {
			return filepath.SkipDir
		}
		if !d.IsDir() && strings.HasSuffix(rel, ".md") {
			files = append(files, rel)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

// guardedLines returns every line of the guarded docs that is not history.
func guardedLines(t *testing.T, root string) []docLine {
	t.Helper()
	var out []docLine
	for _, f := range guardedDocs(t, root) {
		b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(f)))
		if err != nil {
			t.Fatal(err)
		}
		inBlock, inFence := false, false // after an alone marker, until a blank line
		for i, text := range strings.Split(string(b), "\n") {
			if inBlock && strings.HasPrefix(strings.TrimSpace(text), "```") {
				inFence = !inFence
			}
			if strings.TrimSpace(text) == "" && !inFence {
				inBlock = false
			}
			if strings.TrimSpace(text) == historyMarker {
				inBlock = true
			}
			if inBlock || strings.Contains(text, historyMarker) {
				continue
			}
			out = append(out, docLine{f, i + 1, text})
		}
	}
	return out
}

// sourceFiles lists files under root whose name ends in one of exts, skipping
// docs, VCS and dependency folders. Test files are included only when asked.
func sourceFiles(t *testing.T, root string, exts []string, tests bool) []string {
	t.Helper()
	var out []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "node_modules", "vendor", "docs", "testdata", ".claude":
				return filepath.SkipDir
			}
			return nil
		}
		isTest := strings.HasSuffix(p, "_test.go") || strings.Contains(filepath.ToSlash(p), ".test.")
		for _, e := range exts {
			if strings.HasSuffix(p, e) && isTest == tests {
				out = append(out, p)
				break
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func mustRead(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
