// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/tiagovilasboas/downshift/internal/execwrap"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// allowlistedFlag is the read-only query `downshift compact --allowlisted -- <argv>`:
// exit 0 when execwrap names a family for argv, 1 otherwise. It runs nothing,
// prints nothing and logs nothing, so the shell helper from `downshift enable`
// can ask before it wraps. The policy stays in execwrap; the shell holds no list.
const allowlistedFlag = "--allowlisted"

// runExec is `downshift exec -- <program> [args...]`: run the program with no
// shell, keep its exit code, and compact only clean output of an allowlisted
// family (internal/execwrap). DOWNSHIFT_EXEC=0 runs it as a plain
// passthrough. The exec log records the family, byte counts and exit class
// only; a log failure changes nothing the command prints or returns.
//
// With --allowlisted before the `--`, it only answers whether argv is
// allowlisted (see allowlistedFlag).
//
// The session is the hook's --session=<hash>; without it (a direct call by an
// agent or a script, a compound command the hook did not rewrite) it is the
// harness's own session variable, hashed, so the run still counts for the
// session that ran it.
func runExec(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	session := ""
	if len(args) > 0 && strings.HasPrefix(args[0], "--session=") {
		if h := strings.TrimPrefix(args[0], "--session="); telemetry.ValidSessionHash(h) {
			session = h
		}
		args = args[1:]
	}
	if session == "" {
		session = envSessionHash()
	}
	if len(args) > 0 && args[0] == allowlistedFlag {
		return runAllowlistedQuery(args[1:], stderr)
	}
	if len(args) == 0 || args[0] != "--" {
		fmt.Fprintln(stderr, execUsage)
		return 2
	}
	res := execwrap.Run(args[1:], execwrap.Options{
		Stdin:   stdin,
		Stdout:  stdout,
		Stderr:  stderr,
		Compact: os.Getenv(execEnvKillSwitch) != "0" && os.Getenv(execwrap.DepthEnv) == "",
	})
	if len(args) > 1 {
		if path, err := execwrap.EventsPath(); err == nil {
			ev := execwrap.EventFrom(res, time.Now())
			ev.Session = session
			_ = execwrap.Append(path, ev)
		}
	}
	return res.ExitCode
}

const execUsage = "usage: downshift compact [--session=<hash>] [--allowlisted] -- <program> [args...]"

// runAllowlistedQuery answers the --allowlisted query: 0 when argv has an
// allowlisted family, 1 when it has none (an empty argv included), 2 when the
// `--` is missing. It has no side effects: no run, no output, no exec log row.
func runAllowlistedQuery(args []string, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "--" {
		fmt.Fprintln(stderr, execUsage)
		return 2
	}
	if execwrap.FamilyOf(args[1:]) == execwrap.FamilyNone {
		return 1
	}
	return 0
}
