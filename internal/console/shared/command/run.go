// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package command

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"slices"
	"strings"
	"time"
)

// Timeout bounds a captured run so a stuck command never freezes the console.
const Timeout = 60 * time.Second

// Result is the captured outcome of one command run.
type Result struct {
	Args   []string
	Stdout string
	Stderr string
	Code   int
	Err    error // set when the process could not start or timed out
}

// OK reports whether the run started and exited 0.
func (r Result) OK() bool { return r.Err == nil && r.Code == 0 }

// Runner runs Downshift commands as child processes of the binary at Exe.
type Runner struct {
	Exe string
}

// Capture runs args with no stdin and returns its output. Colour is turned off
// in the child so the console decides how text looks.
func (r Runner) Capture(ctx context.Context, args []string) Result {
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, r.Exe, captureArgs(args)...)
	cmd.Env = childEnv("NO_COLOR=1")
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	err := cmd.Run()
	res := Result{Args: args, Stdout: out.String(), Stderr: errb.String()}
	var exitErr *exec.ExitError
	switch {
	case err == nil:
	case errors.As(err, &exitErr):
		res.Code = exitErr.ExitCode()
	default:
		res.Code, res.Err = -1, err
	}
	if ctx.Err() != nil { // timed out, or the caller cancelled
		res.Err = ctx.Err()
	}
	return res
}

// Interactive builds the process for a command that takes over the terminal.
func (r Runner) Interactive(args []string) *exec.Cmd {
	cmd := exec.Command(r.Exe, args...)
	cmd.Env = childEnv()
	return cmd
}

// childEnv is the parent environment plus extra entries, with DOWNSHIFT_PLAIN=1
// appended so every child runs in plain mode and never opens the console TUI.
// Appended entries win over earlier duplicates in exec.Cmd.Env.
func childEnv(extra ...string) []string {
	env := append(os.Environ(), "DOWNSHIFT_PLAIN=1")
	return append(env, extra...)
}

// captureArgs asks the report for its plain text form, which an output pane can
// show; its default is markdown. Any explicit format, or a subcommand such as
// "report clear", is left as typed.
func captureArgs(args []string) []string {
	if len(args) == 0 || args[0] != "report" {
		return args
	}
	for _, a := range args[1:] {
		if !strings.HasPrefix(a, "--days=") {
			return args
		}
	}
	return append(slices.Clone(args), "--text")
}
