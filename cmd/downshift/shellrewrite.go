// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/tiagovilasboas/downshift/internal/contextopt"
	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/execwrap"
	"github.com/tiagovilasboas/downshift/internal/shellwrap"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// execEnvKillSwitch turns the exec wrapper off everywhere when set to "0".
// Kept here so the existing tests in this package can reference it directly.
const execEnvKillSwitch = execwrap.KillSwitchEnv

// execConfigEnabled reports whether the compact wrapper is on: on unless the
// config file says "exec": false. A config that cannot be read counts as on,
// the default. Tests swap it.
var execConfigEnabled = func() bool {
	cfg, err := contextopt.LoadConfig()
	if err != nil {
		return true
	}
	return cfg.ExecOn()
}

// execExecutable is the absolute, symlink-resolved path of the running
// downshift binary, or an error when it is not an executable regular file.
// Tests swap it.
var execExecutable = func() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	if exe, err = filepath.EvalSymlinks(exe); err != nil {
		return "", err
	}
	info, err := os.Stat(exe)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 {
		return "", errors.New("not an executable file")
	}
	return exe, nil
}

func init() {
	// Wire the three env accessors the shellwrap domain reads. Tests swap the
	// package-level vars above before calling shellwrap functions; SetEnvFuncs
	// is re-called in withExec so the domain sees the swapped values.
	shellwrap.SetEnvFuncs(
		func() bool { return execConfigEnabled() },
		func() (string, error) { return execExecutable() },
		func(k string) string { return hookGetenv(k) },
	)
}

// shellWrapDecision is the shared, harness-free decision for one shell command.
// Kept as a package-level function so existing tests in this package can call
// it directly without going through a full event parse.
func shellWrapDecision(harness, command string, blocked func(string) bool) execwrap.Decision {
	if hookGetenv(execEnvKillSwitch) == "0" || hookGetenv(execwrap.DepthEnv) != "" || !execConfigEnabled() {
		return execwrap.Decision{Reason: "disabled"}
	}
	exe, err := execExecutable()
	if err != nil {
		return execwrap.Decision{Reason: "executable_unavailable"}
	}
	d := execwrap.Decide(command, exe)
	if !d.Rewrite {
		return d
	}
	caps, ok := core.CapsFor(harness)
	if !ok || (!caps.CanRewriteShellCommand && !caps.CanAdviseShellWrap) {
		return execwrap.Decision{Family: d.Family, Reason: "harness_cannot_rewrite"}
	}
	if blocked == nil || blocked(command) || blocked(d.Argv) || anyBlocked(blocked, d.Segments) {
		return execwrap.Decision{Family: d.Family, Reason: "blocked_by_user_rules"}
	}
	return d
}

// anyBlocked reports whether a deny or ask rule matches any segment.
func anyBlocked(blocked func(string) bool, segments []string) bool {
	for _, s := range segments {
		if blocked(s) {
			return true
		}
	}
	return false
}

// shellRewrite handles a shell-tool call inside runHookAdapter. It delegates
// entirely to the shellwrap domain. isShell is false when ev is not a shell
// call; out is the wire response, or nil to allow the command unchanged.
func shellRewrite[E any](harness string, ev E) (out any, isShell bool) {
	sessionTag := ""
	if identified, ok := any(ev).(interface{ SessionIdentifier() string }); ok {
		sessionTag = telemetry.HashSessionID(identified.SessionIdentifier())
	}
	r := shellwrap.Handle(harness, ev, sessionTag)
	if !r.IsShell {
		return nil, false
	}
	if r.Rewrite != nil {
		return r.Rewrite, true
	}
	// Advise path: hookrun exits 2 and writes r.Advise to stderr.
	if r.Advise != "" {
		return shellwrap.AdviseResult{Message: r.Advise}, true
	}
	return nil, true
}
