// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package docscheck_test guards the markdown layout: file names follow the
// naming rule in docs/README.md and relative links resolve.
package docscheck_test

import (
	"bufio"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// rootNames are the conventional uppercase community files allowed at the
// repository root.
var rootNames = map[string]bool{
	"README.md": true, "CHANGELOG.md": true, "CONTRIBUTING.md": true,
	"SECURITY.md": true, "CODE_OF_CONDUCT.md": true, "GOVERNANCE.md": true,
	"ROADMAP.md": true, "COMMERCIAL.md": true, "MONETIZATION.md": true,
	"AGENTS.md": true, "CLAUDE.md": true,
}

// toolNames are names a tool requires exactly, allowed in any folder.
var toolNames = map[string]bool{"README.md": true, "SKILL.md": true, "AGENTS.md": true}

// toolPaths are exact paths a tool requires.
var toolPaths = map[string]bool{".github/PULL_REQUEST_TEMPLATE.md": true}

var kebab = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*(\.[a-z0-9]+(-[a-z0-9]+)*)*\.md$`)

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}

// markdownFiles lists tracked and untracked-but-not-ignored .md files,
// slash-separated and relative to root.
func markdownFiles(t *testing.T, root string) []string {
	t.Helper()
	cmd := exec.Command("git", "ls-files", "--cached", "--others", "--exclude-standard", "--", "*.md")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		t.Skipf("git ls-files unavailable: %v", err)
	}
	var files []string
	for _, f := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if f == "" {
			continue
		}
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(f))); err != nil {
			continue // deleted in the working tree
		}
		files = append(files, f)
	}
	return files
}

func TestMarkdownNames(t *testing.T) {
	root := repoRoot(t)
	for _, f := range markdownFiles(t, root) {
		base := path.Base(f)
		switch {
		case !strings.Contains(f, "/") && rootNames[base] || strings.HasPrefix(base, "LICENSE"):
		case toolNames[base], toolPaths[f], kebab.MatchString(base):
		default:
			t.Errorf("%s: markdown names outside the root community files must be lowercase kebab-case", f)
		}
	}
}

var (
	inlineLink = regexp.MustCompile(`\]\(([^)\s]+)(?:\s+"[^"]*")?\)`)
	refDef     = regexp.MustCompile(`^\s{0,3}\[[^\]]+\]:\s*(\S+)`)
	inlineCode = regexp.MustCompile("`[^`]*`")
	scheme     = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9+.-]*:`)
)

func TestRelativeLinksResolve(t *testing.T) {
	root := repoRoot(t)
	for _, f := range markdownFiles(t, root) {
		fh, err := os.Open(filepath.Join(root, filepath.FromSlash(f)))
		if err != nil {
			t.Fatal(err)
		}
		sc := bufio.NewScanner(fh)
		sc.Buffer(make([]byte, 1024*1024), 1024*1024)
		fence, n := "", 0
		for sc.Scan() {
			n++
			line := sc.Text()
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
				if fence == "" {
					fence = trimmed[:3]
				} else if strings.HasPrefix(trimmed, fence) {
					fence = ""
				}
				continue
			}
			if fence != "" {
				continue
			}
			line = inlineCode.ReplaceAllString(line, "")
			var targets []string
			for _, m := range inlineLink.FindAllStringSubmatch(line, -1) {
				targets = append(targets, m[1])
			}
			if m := refDef.FindStringSubmatch(line); m != nil {
				targets = append(targets, m[1])
			}
			for _, target := range targets {
				if scheme.MatchString(target) || strings.HasPrefix(target, "#") || strings.HasPrefix(target, "/") || strings.HasPrefix(target, "<") {
					continue
				}
				p, _, _ := strings.Cut(target, "#")
				p, _, _ = strings.Cut(p, "?")
				if p == "" {
					continue
				}
				resolved := path.Join(path.Dir(f), p)
				if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(resolved))); err != nil {
					t.Errorf("%s:%d: relative link %q does not resolve", f, n, target)
				}
			}
		}
		fh.Close()
		if err := sc.Err(); err != nil {
			t.Fatalf("%s: %v", f, err)
		}
	}
}
