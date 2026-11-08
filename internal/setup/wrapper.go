// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package setup

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/tiagovilasboas/downshift-ai/internal/setup/configfile"
)

// Wrapper hooks. KiroCrew registers a hook as one absolute executable path with
// no arguments, so it cannot run `downshift kirocrew` directly. A Hook that
// names a Wrapper stem is carried by a small script that `setup` writes under
// the state directory and that execs the running binary with the subcommand.
// Generating it (rather than shipping scripts/) is what makes it work for an
// install that has no repository checkout, and for the binary's real path.
//
// Only this file and internal/setup/wrapper_name.go know the host OS differs:
// the script's quoting and line ending are decided here, its extension there,
// with the OS passed in so both forms are tested on any host.

// wrapperScript renders the wrapper that runs `<binary> <args>` and passes the
// hook payload (stdin) and the exit code through. The recorded binary is used
// while it exists; when it has moved or been reinstalled, the POSIX script falls
// back to the `downshift` on PATH. If neither exists, the call goes through
// (exit 0) and says so on stderr: the router never blocks on its own doubt, but
// it must not fail silently. Because the hook chain also runs the
// operator's own guards, that is a deliberate trade: a vanished router cannot
// stop their guards from running, only stop being a router.
//
// binary is quoted for the target shell; args is written as is, so it must be a
// constant subcommand from the setup table (Hook.Args), never data.
func wrapperScript(goos, binary, args string) (string, error) {
	if strings.ContainsAny(binary, "\r\n\x00") {
		return "", errors.New("the binary path contains a control character")
	}
	if goos == "windows" {
		if strings.Contains(binary, `"`) {
			return "", errors.New(`the binary path contains a double quote`)
		}
		q := `"` + strings.ReplaceAll(binary, "%", "%%") + `"`
		lines := []string{
			"@echo off",
			"rem Copyright (c) 2026 Tiago Vilas Boas.",
			"rem SPDX-License-Identifier: Apache-2.0",
			"rem Downshift by Tiago Vilas Boas",
			"rem https://github.com/tiagovilasboas/downshift-ai",
			"rem " + configfile.OwnedPhrase + ", which rewrites it. Edit nothing here.",
			"rem The harness takes one executable and no arguments, so this carries the subcommand.",
			"if not exist " + q + " (",
			"  echo downshift: binary not found, hook skipped 1>&2",
			"  exit /b 0",
			")",
			q + " " + args,
			"exit /b %ERRORLEVEL%",
		}
		return strings.Join(lines, "\r\n") + "\r\n", nil
	}
	q := "'" + strings.ReplaceAll(binary, "'", `'\''`) + "'"
	lines := []string{
		"#!/bin/sh",
		"# Copyright (c) 2026 Tiago Vilas Boas.",
		"# SPDX-License-Identifier: Apache-2.0",
		"# Downshift by Tiago Vilas Boas",
		"# https://github.com/tiagovilasboas/downshift-ai",
		"# " + configfile.OwnedPhrase + ", which rewrites it. Edit nothing here.",
		"# The harness takes one executable and no arguments, so this carries the subcommand.",
		"# The recorded binary wins while it exists; otherwise the downshift on PATH.",
		"recorded=" + q,
		`bin=$recorded`,
		`if [ ! -x "$bin" ]; then bin=$(command -v downshift 2>/dev/null); fi`,
		`[ -n "$bin" ] && [ -x "$bin" ] || { echo "downshift: $recorded not found and no downshift on PATH, hook skipped" >&2; exit 0; }`,
		`exec "$bin" ` + args,
	}
	return strings.Join(lines, "\n") + "\n", nil
}

// planWrapper plans the wrapper file for h. Like planSteering it reuses
// Plan, so the diff, the backup and --undo behave as for a hook file.
func planWrapper(o Options, h Hook) Plan {
	p := Plan{Path: WrapperPath(o.StatePath(), h), Mode: 0o700}
	body, err := wrapperScript(runtime.GOOS, o.Binary, h.Args)
	if err != nil {
		p.Err = fmt.Errorf("cannot generate the %s wrapper: %w", h.Wrapper, err)
		return p
	}
	p.After = []byte(body)
	if b, err := os.ReadFile(p.Path); err == nil {
		p.Before = b
		p.Existed = true
	} else if !os.IsNotExist(err) {
		p.Err = err
	}
	return p
}

// failedCompanion returns the path of a companion of p that could not be
// written, or "" when p can be written.
func failedCompanion(p Plan, unwritable map[string]bool) string {
	for _, c := range p.Companions {
		if unwritable[c.Path] {
			return c.Path
		}
	}
	return ""
}

// wrapperPathsFor lists the wrapper files setup keeps for t, so --undo can
// restore or remove them with the config that points at them.
func wrapperPathsFor(o Options, t Target) []string {
	var out []string
	for _, h := range t.Hooks {
		if h.Wrapper != "" {
			out = append(out, WrapperPath(o.StatePath(), h))
		}
	}
	return out
}
