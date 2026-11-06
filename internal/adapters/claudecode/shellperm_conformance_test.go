// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package claudecode

import "testing"

// Conformance of bashRuleMatches with the Bash rule examples in the official
// Claude Code documentation:
//
//	https://code.claude.com/docs/en/permissions
//	sections "Match all uses of a tool", "Use specifiers for fine-grained
//	control" and "Wildcard patterns" (accessed 2026-10-21).
//
// Every row is copied from the docs ("You write" / "Matches" / "Doesn't
// match"). Only the command text is matched here: compound-command splitting,
// wrapper stripping and leading env assignments are out of scope, because the
// Compactor rewrites single allowlisted commands only and also checks the
// normalised argv (see cmd/downshift/shellrewrite.go).
func TestBashRuleMatches_ClaudeCodeDocsConformance(t *testing.T) {
	rows := []struct {
		rule     string
		matches  []string
		excludes []string
	}{
		// Match all uses of a tool; `Bash(*)` is equivalent to `Bash`.
		{"Bash", []string{"npm run build", "git push origin main"}, nil},
		{"Bash(*)", []string{"npm run build", "git push origin main"}, nil},
		// Wildcard patterns table.
		{"Bash(npm run build)", []string{"npm run build"}, []string{"npm run build --watch"}},
		{"Bash(npm run *)", []string{"npm run build", "npm run test --watch", "npm run"}, []string{"npm install"}},
		{"Bash(git log * main)", []string{"git log --oneline main", "git log -5 main", "git log --output=<file> main"}, []string{"git log main", "git push origin main"}},
		{"Bash(git * main)", []string{"git merge main", "git push origin main", "git -c core.fsmonitor=<script> diff main"}, []string{"git log"}},
		{"Bash(* --version)", []string{"node --version", "bash -c 'echo hi' --version"}, []string{"node -v"}},
		{"Bash(ls *)", []string{"ls -la", "ls"}, []string{"lsof"}},
		{"Bash(ls*)", []string{"ls -la", "lsof"}, nil},
		{"Bash(* --help *)", []string{"npm --help x"}, []string{"npm --help"}},
		// "The `:*` suffix is an equivalent way to write a trailing wildcard."
		{"Bash(ls:*)", []string{"ls -la", "ls"}, []string{"lsof"}},
		// "In a pattern like `Bash(git:* push)`, the colon is treated as a
		// literal character and won't match git commands."
		{"Bash(git:* push)", nil, []string{"git push", "git status push"}},
		// Settings example: deny `Bash(git push *)` refuses commands that
		// begin with `git push`.
		{"Bash(git push *)", []string{"git push", "git push origin main"}, []string{"git pull"}},
	}
	for _, r := range rows {
		for _, c := range r.matches {
			if !bashRuleMatches(r.rule, c) {
				t.Errorf("%s must match %q (docs)", r.rule, c)
			}
		}
		for _, c := range r.excludes {
			if bashRuleMatches(r.rule, c) {
				t.Errorf("%s must not match %q (docs)", r.rule, c)
			}
		}
	}
}
