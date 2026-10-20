// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
)

// Wrapper hooks. KiroCrew registers a hook as one absolute executable path with
// no arguments, so it cannot run `downshift kirocrew` directly. A setupHook that
// names a Wrapper stem is carried by a small script that `setup` writes under
// the state directory and that execs the running binary with the subcommand.
// Generating it (rather than shipping scripts/) is what makes it work for an
// install that has no repository checkout, and for the binary's real path.
//
// This file is the only place that knows the host OS differs: the script's
// extension, quoting and line ending are decided here, with the OS passed in so
// both forms are tested on any host.

const wrapperDirName = "bin"

// wrapperExt is the script extension of the host OS.
func wrapperExt(goos string) string {
	if goos == "windows" {
		return ".cmd"
	}
	return ".sh"
}

// wrapperFile is the file name of the wrapper with the given stem on goos.
func wrapperFile(stem, goos string) string { return stem + wrapperExt(goos) }

// isWrapperCommand reports whether command runs a wrapper with this stem from
// any directory, for either OS. A wrapper takes no arguments, so the whole
// string is the path. Used to recognise a hook the user registered by hand.
func isWrapperCommand(command, stem string) bool {
	base := path.Base(strings.ReplaceAll(strings.TrimSpace(command), `\`, "/"))
	ext := path.Ext(base)
	return (ext == ".sh" || ext == ".cmd") && strings.TrimSuffix(base, ext) == stem
}

// ownsCommand reports whether command is the hook h written by a previous run
// or by hand: a wrapper in any directory, or `downshift <args>` through any
// binary path.
func ownsCommand(command string, h setupHook) bool {
	if h.Wrapper != "" {
		return isWrapperCommand(command, h.Wrapper)
	}
	return isDownshiftCommand(command, h.Args)
}

// wrapperPath is where setup keeps the wrapper for h.
func wrapperPath(stateDir string, h setupHook) string {
	return filepath.Join(stateDir, wrapperDirName, wrapperFile(h.Wrapper, runtime.GOOS))
}

// wrapperScript renders the wrapper that runs `<binary> <args>` and passes the
// hook payload (stdin) and the exit code through. A missing binary lets the
// call through (exit 0) and says so on stderr: the router never blocks on its
// own doubt, but it must not fail silently. Because the hook chain also runs the
// operator's own guards, that is a deliberate trade: a vanished router cannot
// stop their guards from running, only stop being a router.
//
// binary is quoted for the target shell; args is written as is, so it must be a
// constant subcommand from the setup table (setupHook.Args), never data.
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
			"rem https://github.com/tiagovilasboas/downshift",
			"rem Written by `downshift setup`, which rewrites it. Edit nothing here.",
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
		"# https://github.com/tiagovilasboas/downshift",
		"# Written by `downshift setup`, which rewrites it. Edit nothing here.",
		"# The harness takes one executable and no arguments, so this carries the subcommand.",
		"bin=" + q,
		`[ -x "$bin" ] || { echo "downshift: $bin not found, hook skipped" >&2; exit 0; }`,
		`exec "$bin" ` + args,
	}
	return strings.Join(lines, "\n") + "\n", nil
}

// planWrapper plans the wrapper file for h. Like planSteering it reuses
// setupPlan, so the diff, the backup and --undo behave as for a hook file.
func planWrapper(o setupOptions, h setupHook) setupPlan {
	p := setupPlan{path: wrapperPath(o.stateDir(), h), mode: 0o700}
	body, err := wrapperScript(runtime.GOOS, o.Binary, h.Args)
	if err != nil {
		p.err = fmt.Errorf("cannot generate the %s wrapper: %w", h.Wrapper, err)
		return p
	}
	p.after = []byte(body)
	if b, err := os.ReadFile(p.path); err == nil {
		p.before = b
		p.existed = true
	} else if !os.IsNotExist(err) {
		p.err = err
	}
	return p
}

// failedCompanion returns the path of a companion of p that could not be
// written, or "" when p can be written.
func failedCompanion(p setupPlan, unwritable map[string]bool) string {
	for _, c := range p.companions {
		if unwritable[c.path] {
			return c.path
		}
	}
	return ""
}

// wrapperPathsFor lists the wrapper files setup keeps for t, so --undo can
// restore or remove them with the config that points at them.
func wrapperPathsFor(o setupOptions, t setupTarget) []string {
	var out []string
	for _, h := range t.Hooks {
		if h.Wrapper != "" {
			out = append(out, wrapperPath(o.stateDir(), h))
		}
	}
	return out
}
