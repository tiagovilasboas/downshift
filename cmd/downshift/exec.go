// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"github.com/tiagovilasboas/downshift-ai/internal/core"
	"io"
	"os"
	"strings"
	"time"

	"github.com/tiagovilasboas/downshift-ai/internal/cascade"
	"github.com/tiagovilasboas/downshift-ai/internal/execwrap"
	"github.com/tiagovilasboas/downshift-ai/internal/telemetry"
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
// agent or a script, a compound command the hook did not rewrite) there is no
// session fallback to a harness env var: a direct call carries no session
// unless --session is given. This avoids attributing direct calls to a harness
// they did not originate from (audit A22). --harness=<name> names the harness
// when its shell exports no session variable: the hook rewrite and the
// steering setup writes both add it; an unknown name is ignored.
func runExec(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	session, harness, entry := "", "", "direct"
	for len(args) > 0 {
		if h, ok := strings.CutPrefix(args[0], "--session="); ok {
			if telemetry.ValidSessionHash(h) {
				session, entry = h, "hook"
			}
		} else if h, ok := strings.CutPrefix(args[0], "--harness="); ok {
			if _, known := core.CapsFor(h); known {
				harness = h
			}
		} else {
			break
		}
		args = args[1:]
	}
	if harness == "" {
		harness = envHarness()
	}
	if len(args) > 0 && args[0] == allowlistedFlag {
		return runAllowlistedQuery(args[1:], stderr)
	}
	var opts execwrap.Options
	if args = execwrap.ParseTailFlags(args, &opts); len(args) == 0 || args[0] != "--" {
		fmt.Fprintln(stderr, execUsage)
		return 2
	}
	opts.Stdin, opts.Stdout, opts.Stderr = stdin, stdout, stderr
	opts.Compact = os.Getenv(execEnvKillSwitch) != "0" && os.Getenv(execwrap.DepthEnv) == ""
	opts.Keep, opts.KeepFailed = ccrKeep()
	loadBalanced().apply(&opts)
	argv := execwrap.SplitSingleWord(args[1:])
	opts.Desktop = desktopStep(session, argv)
	res := execwrap.Run(argv, opts)
	if len(args) > 1 {
		if path, err := execwrap.EventsPath(); err == nil {
			ev := execwrap.EventFrom(res, time.Now())
			ev.Session = session
			ev.Harness = harness
			ev.Model = envModel(ev.Harness)
			ev.Entry = entry
			ev.Client = envClient()
			_ = execwrap.Append(path, ev)
		}
		if res.ExitCode != 0 {
			cascade.RecordFailure(session, strings.Join(args[1:], " "), envModel(harness), fmt.Sprintf("command exited with code %d", res.ExitCode))
		}
	}
	return execwrap.ShellExit(opts, res.ExitCode)
}

const execUsage = "usage: downshift compact [--session=<hash>] [--harness=<name>] [--allowlisted] [--discard-stderr] [--exit-zero] -- <program> [args...]"

// runAllowlistedQuery answers the --allowlisted query: 0 when argv has an
// allowlisted family, 1 when it has none (an empty argv included), 2 when the
// `--` is missing. It has no side effects: no run, no output, no exec log row.
func runAllowlistedQuery(args []string, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "--" {
		fmt.Fprintln(stderr, execUsage)
		return 2
	}
	if execwrap.FamilyOf(execwrap.SplitSingleWord(args[1:])) == execwrap.FamilyNone {
		return 1
	}
	return 0
}
