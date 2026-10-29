// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package worktree

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// maxStderr bounds the stderr text kept in an ExitError. Git can echo branch
// or path names derived from task text, so only a short first line is kept.
const maxStderr = 200

// dropped lists the inherited variables the runner replaces. The GIT_* names
// point git at a repository, so they are removed and each call runs in the
// directory it is given. LC_ALL, GIT_TERMINAL_PROMPT and GIT_OPTIONAL_LOCKS
// are fixed by the runner. Keys are upper case and matched case-insensitively
// so that Windows environment names are handled too.
var dropped = map[string]bool{
	"GIT_DIR":                          true,
	"GIT_WORK_TREE":                    true,
	"GIT_INDEX_FILE":                   true,
	"GIT_COMMON_DIR":                   true,
	"GIT_OBJECT_DIRECTORY":             true,
	"GIT_ALTERNATE_OBJECT_DIRECTORIES": true,
	"GIT_PREFIX":                       true,
	"LC_ALL":                           true,
	"GIT_TERMINAL_PROMPT":              true,
	"GIT_OPTIONAL_LOCKS":               true,
}

// Result holds what git wrote to its standard streams.
type Result struct {
	Stdout string
	Stderr string
}

// ExitError reports a git command that exited with a non-zero status. It names
// the subcommand only, never the full argument list.
type ExitError struct {
	Subcommand string
	Code       int
	Stderr     string // first line of stderr, bounded by maxStderr
}

func (e *ExitError) Error() string {
	if e.Stderr == "" {
		return fmt.Sprintf("git %s: exit status %d", e.Subcommand, e.Code)
	}
	return fmt.Sprintf("git %s: exit status %d: %s", e.Subcommand, e.Code, e.Stderr)
}

// Run executes git with args in dir. Use it for commands that may write to the
// repository. Paths must follow "--"; see WithPaths.
func Run(ctx context.Context, dir string, args ...string) (Result, error) {
	return execGit(ctx, dir, args, false)
}

// RunReadOnly executes git with args in dir and sets GIT_OPTIONAL_LOCKS=0, so
// an inspection never takes optional index or ref locks.
func RunReadOnly(ctx context.Context, dir string, args ...string) (Result, error) {
	return execGit(ctx, dir, args, true)
}

// WithPaths returns a copy of args followed by "--" and paths, so that no path
// can be read as an option. With no paths it returns a copy of args unchanged.
func WithPaths(args []string, paths ...string) []string {
	out := make([]string, 0, len(args)+1+len(paths))
	out = append(out, args...)
	if len(paths) == 0 {
		return out
	}
	out = append(out, "--")
	return append(out, paths...)
}

func execGit(ctx context.Context, dir string, args []string, readOnly bool) (Result, error) {
	if len(args) == 0 {
		return Result{}, errors.New("git: no subcommand given")
	}
	if dir == "" {
		return Result{}, errors.New("git: no working directory given")
	}
	sub := args[0]
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	cmd.Env = buildEnv(os.Environ(), readOnly)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	res := Result{}
	err := cmd.Run()
	res.Stdout = stdout.String()
	res.Stderr = stderr.String()
	if err == nil {
		return res, nil
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return res, fmt.Errorf("git %s: %w", sub, ctxErr)
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return res, &ExitError{Subcommand: sub, Code: exitErr.ExitCode(), Stderr: boundStderr(stderr.Bytes())}
	}
	return res, fmt.Errorf("git %s: %w", sub, err)
}

// buildEnv returns base without the dropped variables, plus the fixed ones.
// Git never prompts for credentials, and read-only calls skip optional locks.
func buildEnv(base []string, readOnly bool) []string {
	env := make([]string, 0, len(base)+3)
	for _, kv := range base {
		key, _, _ := strings.Cut(kv, "=")
		if dropped[strings.ToUpper(key)] {
			continue
		}
		env = append(env, kv)
	}
	env = append(env, "LC_ALL=C", "GIT_TERMINAL_PROMPT=0")
	if readOnly {
		env = append(env, "GIT_OPTIONAL_LOCKS=0")
	}
	return env
}

// boundStderr returns the first non-empty line of stderr, cut to maxStderr
// bytes. A partial UTF-8 sequence at the cut is dropped.
func boundStderr(b []byte) string {
	first, _, _ := strings.Cut(strings.TrimSpace(string(b)), "\n")
	first = strings.TrimSpace(first)
	if len(first) > maxStderr {
		return strings.ToValidUTF8(first[:maxStderr], "") + "..."
	}
	return first
}
