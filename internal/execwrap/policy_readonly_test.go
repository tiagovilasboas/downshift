// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import "testing"

func TestReadOnly(t *testing.T) {
	cases := []struct {
		command string
		want    bool
	}{
		// Read-only, one command, no metacharacter.
		{"git status", true},
		{"  git status --short  ", true},
		{"git log --oneline -n 5", true},
		{"git diff --stat", true},
		{"git show HEAD", true},
		{"git branch", true},
		{"git branch -a -v", true},
		{"git branch --show-current", true},
		{"git rev-parse HEAD", true},
		{"git ls-files", true},
		{"git remote -v", true},
		{"git stash list", true},
		{"gh pr view 12", true},
		{"gh pr list", true},
		{"gh pr checks 12", true},
		{"gh issue list", true},
		{"gh run view 99", true},
		{"gh repo view", true},
		{"go version", true},
		{"go env GOPATH", true},
		{"go list ./...", true},
		{"go doc fmt.Println", true},
		{"mise list", true},
		{"asdf current", true},
		{"direnv status", true},
		{"ls -la internal", true},

		// Chained or with any shell metacharacter.
		{"go version ; perl /tmp/x.pl", false},
		{"git status && rm -rf x", false},
		{"git status || true", false},
		{"git status | cat", false},
		{"git status &", false},
		{"git status $(id)", false},
		{"git status `id`", false},
		{"git status > out", false},
		{"git status < in", false},
		{"git status\nperl x", false},
		{"git status\tx", false},
		{"cd sub && git status", false},

		// Env assignments can redirect a program to run code.
		{"GIT_EXTERNAL_DIFF=/tmp/x git diff", false},

		// Runners, builds, tests and installs execute project code.
		{"make check", false},
		{"go test ./...", false},
		{"go run .", false},
		{"go build ./...", false},
		{"go generate ./...", false},
		{"uv run pytest", false},
		{"npm test", false},
		{"pytest", false},
		{"cargo test", false},

		// Mutating subcommands and flags.
		{"git pull", false},
		{"git commit -m x", false},
		{"git branch -D main", false},
		{"git branch new", false},
		{"git stash", false},
		{"git stash pop", false},
		{"git remote add x y", false},
		{"git diff --output=/tmp/x", false},
		{"git diff --ext-diff", false},
		{"git log --textconv", false},
		{"git checkout main", false},
		{"gh pr merge 12", false},
		{"gh pr checkout 12", false},
		{"gh issue close 3", false},
		{"gh run rerun 99", false},
		{"gh api repos/x/y", false},
		{"gh auth status", false},
		{"gh repo delete x", false},
		{"go env -w GOFLAGS=-x", false},
		{"go env -u GOFLAGS", false},
		{"go list -toolexec=/tmp/x ./...", false},
		{"go list --toolexec /tmp/x ./...", false},
		{"go list -mod=mod ./...", false},
		{"mise use node@20", false},
		{"mise install", false},
		{"asdf global node 20", false},
		{"direnv allow", false},
		{"aws s3 cp a b", false},
		{"kubectl get pods", false},

		// Security review of the read-only set.
		{"go tool compile -o x.o m.go", false},
		{"go tool mytool", false},
		{"go tool pprof -http=:8080 x", false},
		{"go doc -http", false},
		{"go doc --http=:6060", false},
		{"git stash list --output=/tmp/x", false},
		{"git stash list -p --ext-diff", false},
		{"git stash list -p", true},
		{"gh repo view --web", false},
		{"gh pr view 1 -w", false},
		{"gh search repos x --web", false},
		{"gh search repos x", true},

		// Edge shapes.
		{"", false},
		{"git", false},
		{"downshift compact -- git status", false},
	}
	for _, c := range cases {
		t.Run(c.command, func(t *testing.T) {
			if got := ReadOnly(c.command); got != c.want {
				t.Fatalf("ReadOnly(%q) = %v, want %v", c.command, got, c.want)
			}
		})
	}
}
