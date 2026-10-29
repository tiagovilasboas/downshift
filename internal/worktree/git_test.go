// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package worktree

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// requireGitOnPath skips a test that needs real git when git is not installed.
func requireGitOnPath(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("skipping: git is not on PATH, so the real-git cases cannot run")
	}
}

// initTestRepo creates an empty repository in a fresh temp dir, through the runner.
func initTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if _, err := Run(context.Background(), dir, "init", "-q"); err != nil {
		t.Fatalf("git init: %v", err)
	}
	return dir
}

// canonicalDir resolves symlinks (macOS temp dirs live under /var, a link to
// /private/var) and uses slashes, which is how git prints paths.
func canonicalDir(t *testing.T, dir string) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatalf("resolve %s: %v", dir, err)
	}
	return filepath.ToSlash(resolved)
}

func TestRunOperatesOnGivenDirectory(t *testing.T) {
	requireGitOnPath(t)
	repo := initTestRepo(t)
	root := canonicalDir(t, repo)
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "toplevel", args: []string{"rev-parse", "--show-toplevel"}, want: root},
		{name: "absolute git dir", args: []string{"rev-parse", "--absolute-git-dir"}, want: root + "/.git"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			res, err := RunReadOnly(context.Background(), repo, tc.args...)
			if err != nil {
				t.Fatalf("RunReadOnly(%v): %v", tc.args, err)
			}
			if got := strings.TrimSpace(res.Stdout); got != tc.want {
				t.Errorf("stdout = %q, want %q", got, tc.want)
			}
		})
	}
}

// GIT_DIR and friends point at another repository. The runner must drop them,
// so reads resolve to the directory passed in and writes land there too.
// Without the stripping, rev-parse would print the other repository's git dir.
func TestRunIgnoresInheritedRepositoryVariables(t *testing.T) {
	requireGitOnPath(t)
	ctx := context.Background()
	repo := initTestRepo(t)
	other := initTestRepo(t)
	otherGitDir := filepath.Join(other, ".git")
	t.Setenv("GIT_DIR", otherGitDir)
	t.Setenv("GIT_WORK_TREE", other)
	t.Setenv("GIT_INDEX_FILE", filepath.Join(otherGitDir, "index"))
	t.Setenv("GIT_COMMON_DIR", otherGitDir)
	t.Setenv("GIT_OBJECT_DIRECTORY", filepath.Join(otherGitDir, "objects"))
	t.Setenv("GIT_PREFIX", "sub/")

	runners := []struct {
		name string
		run  func(context.Context, string, ...string) (Result, error)
	}{
		{name: "Run", run: Run},
		{name: "RunReadOnly", run: RunReadOnly},
	}
	for _, r := range runners {
		t.Run(r.name, func(t *testing.T) {
			res, err := r.run(ctx, repo, "rev-parse", "--absolute-git-dir")
			if err != nil {
				t.Fatalf("rev-parse: %v", err)
			}
			want := canonicalDir(t, repo) + "/.git"
			if got := strings.TrimSpace(res.Stdout); got != want {
				t.Errorf("git dir = %q, want %q (inherited GIT_DIR leaked)", got, want)
			}
		})
	}

	if err := os.WriteFile(filepath.Join(repo, "f.txt"), []byte("x\n"), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	if _, err := Run(ctx, repo, WithPaths([]string{"add"}, "f.txt")...); err != nil {
		t.Fatalf("add: %v", err)
	}
	inRepo, err := RunReadOnly(ctx, repo, "ls-files")
	if err != nil {
		t.Fatalf("ls-files in repo: %v", err)
	}
	if got := strings.TrimSpace(inRepo.Stdout); got != "f.txt" {
		t.Errorf("repo index = %q, want %q", got, "f.txt")
	}
	inOther, err := RunReadOnly(ctx, other, "ls-files")
	if err != nil {
		t.Fatalf("ls-files in other: %v", err)
	}
	if got := strings.TrimSpace(inOther.Stdout); got != "" {
		t.Errorf("other index = %q, want empty: write leaked into the inherited repository", got)
	}
}

func TestWithPaths(t *testing.T) {
	tests := []struct {
		name  string
		args  []string
		paths []string
		want  []string
	}{
		{name: "no paths", args: []string{"status", "--porcelain"}, want: []string{"status", "--porcelain"}},
		{name: "one path", args: []string{"add"}, paths: []string{"-n.txt"}, want: []string{"add", "--", "-n.txt"}},
		{name: "several paths", args: []string{"checkout"}, paths: []string{"a", "b"}, want: []string{"checkout", "--", "a", "b"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := WithPaths(tc.args, tc.paths...)
			if strings.Join(got, "\x00") != strings.Join(tc.want, "\x00") {
				t.Errorf("WithPaths() = %q, want %q", got, tc.want)
			}
		})
	}
}

// A path that starts with a dash only works after "--". Without it git would
// read the name as an unknown option.
func TestRunTreatsDashPathAsPath(t *testing.T) {
	requireGitOnPath(t)
	ctx := context.Background()
	repo := initTestRepo(t)
	if err := os.WriteFile(filepath.Join(repo, "-dash.txt"), []byte("x\n"), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	if _, err := Run(ctx, repo, WithPaths([]string{"add"}, "-dash.txt")...); err != nil {
		t.Fatalf("add -dash.txt: %v", err)
	}
	res, err := RunReadOnly(ctx, repo, "ls-files")
	if err != nil {
		t.Fatalf("ls-files: %v", err)
	}
	if got := strings.TrimSpace(res.Stdout); got != "-dash.txt" {
		t.Errorf("ls-files = %q, want %q", got, "-dash.txt")
	}
}

func TestRunReturnsBoundedExitError(t *testing.T) {
	requireGitOnPath(t)
	repo := initTestRepo(t)
	_, err := RunReadOnly(context.Background(), repo, "cat-file", "-e", "HEAD:missing.txt")

	var exitErr *ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("error = %v, want *ExitError", err)
	}
	if exitErr.Subcommand != "cat-file" {
		t.Errorf("Subcommand = %q, want %q", exitErr.Subcommand, "cat-file")
	}
	if exitErr.Code == 0 {
		t.Errorf("Code = 0, want non-zero for a failed command")
	}
	if len(exitErr.Stderr) > maxStderr+len("...") {
		t.Errorf("Stderr is %d bytes, want at most %d", len(exitErr.Stderr), maxStderr+len("..."))
	}
	if !strings.HasPrefix(err.Error(), "git cat-file: exit status ") {
		t.Errorf("Error() = %q, want the subcommand and exit status only", err.Error())
	}
}

func TestBoundStderr(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "empty", in: "", want: ""},
		{name: "whitespace only", in: " \n\t", want: ""},
		{name: "first line only", in: "fatal: bad\nhint: more\n", want: "fatal: bad"},
		{name: "crlf", in: "fatal: bad\r\n", want: "fatal: bad"},
		{name: "long line cut", in: strings.Repeat("a", 500), want: strings.Repeat("a", maxStderr) + "..."},
		{name: "partial rune dropped", in: "a" + strings.Repeat("é", 150), want: "a" + strings.Repeat("é", 99) + "..."},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := boundStderr([]byte(tc.in)); got != tc.want {
				t.Errorf("boundStderr() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestBuildEnv(t *testing.T) {
	base := []string{
		"PATH=/usr/bin",
		"HOME=/home/tester",
		"GIT_CONFIG_GLOBAL=/etc/gitconfig.custom",
		"GIT_DIR=/elsewhere/.git",
		"git_work_tree=/elsewhere",
		"GIT_INDEX_FILE=/elsewhere/index",
		"GIT_COMMON_DIR=/elsewhere/common",
		"GIT_OBJECT_DIRECTORY=/elsewhere/objects",
		"GIT_ALTERNATE_OBJECT_DIRECTORIES=/elsewhere/alt",
		"GIT_PREFIX=sub/",
		"LC_ALL=en_US.UTF-8",
		"GIT_TERMINAL_PROMPT=1",
		"GIT_OPTIONAL_LOCKS=1",
	}
	tests := []struct {
		name     string
		readOnly bool
		present  []string // exact KEY=value entries that must be present
		absent   []string // entries that must not be present
	}{
		{
			name:     "mutating command",
			readOnly: false,
			present:  []string{"PATH=/usr/bin", "HOME=/home/tester", "GIT_CONFIG_GLOBAL=/etc/gitconfig.custom", "LC_ALL=C", "GIT_TERMINAL_PROMPT=0"},
			absent:   []string{"GIT_DIR=/elsewhere/.git", "git_work_tree=/elsewhere", "GIT_INDEX_FILE=/elsewhere/index", "GIT_COMMON_DIR=/elsewhere/common", "GIT_OBJECT_DIRECTORY=/elsewhere/objects", "GIT_ALTERNATE_OBJECT_DIRECTORIES=/elsewhere/alt", "GIT_PREFIX=sub/", "LC_ALL=en_US.UTF-8", "GIT_TERMINAL_PROMPT=1", "GIT_OPTIONAL_LOCKS=1", "GIT_OPTIONAL_LOCKS=0"},
		},
		{
			name:     "read-only command",
			readOnly: true,
			present:  []string{"PATH=/usr/bin", "LC_ALL=C", "GIT_TERMINAL_PROMPT=0", "GIT_OPTIONAL_LOCKS=0"},
			absent:   []string{"GIT_DIR=/elsewhere/.git", "GIT_PREFIX=sub/", "GIT_OPTIONAL_LOCKS=1", "LC_ALL=en_US.UTF-8", "GIT_TERMINAL_PROMPT=1"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			env := buildEnv(base, tc.readOnly)
			for _, want := range tc.present {
				if !slices.Contains(env, want) {
					t.Errorf("env is missing %q", want)
				}
			}
			for _, bad := range tc.absent {
				if slices.Contains(env, bad) {
					t.Errorf("env still has %q", bad)
				}
			}
		})
	}
}

func TestRunRejectsInvalidCalls(t *testing.T) {
	tests := []struct {
		name string
		dir  string
		args []string
	}{
		{name: "no subcommand", dir: t.TempDir(), args: nil},
		{name: "empty directory", dir: "", args: []string{"status"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Run(context.Background(), tc.dir, tc.args...); err == nil {
				t.Error("Run() returned nil error, want an error")
			}
		})
	}
}

func TestRunReportsCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := RunReadOnly(ctx, t.TempDir(), "status")
	if !errors.Is(err, context.Canceled) {
		t.Errorf("error = %v, want context.Canceled", err)
	}
}

func TestRunReportsMissingGit(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	_, err := Run(context.Background(), t.TempDir(), "status")
	if !errors.Is(err, exec.ErrNotFound) {
		t.Errorf("error = %v, want exec.ErrNotFound", err)
	}
}
