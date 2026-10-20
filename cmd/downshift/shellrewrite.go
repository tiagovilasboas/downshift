// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"
	"github.com/tiagovilasboas/downshift/internal/contextopt"
	"os"
	"path/filepath"

	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/execwrap"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// shellEvent is the optional adapter seam for the opt-in exec wrapper. An
// adapter whose harness has a shell tool says which call is that tool and
// how to emit a rewritten command; the decision stays here.
type shellEvent interface {
	// ShellCommand returns the command and true when the call is the
	// harness's shell tool.
	ShellCommand() (command string, isShell bool)
	// ShellRewrite returns the wire output that replaces the command,
	// preserving every other input field.
	ShellRewrite(command string) (any, error)
	// ShellBlocked reports whether the harness's own permission rules forbid
	// rewriting the command: a deny or ask rule matches it. Any rule-reading
	// error is true (fail closed).
	ShellBlocked(command string) bool
}

// execEnvKillSwitch turns the exec wrapper off everywhere when set to "0".
const execEnvKillSwitch = "DOWNSHIFT_EXEC"

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

// shellWrapDecision is the shared, harness-free decision for one shell
// command: kill switch, config, policy, the harness's declared capability,
// the user's own deny and ask rules, and the running binary's absolute path.
// Anything short of all of them leaves the command unchanged.
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
	if caps, ok := core.CapsFor(harness); !ok || !caps.CanRewriteShellCommand {
		return execwrap.Decision{Family: d.Family, Reason: "harness_cannot_rewrite"}
	}
	// The user's own deny and ask rules decide: the original command, as
	// written and as normalised, must match none of them. The rewritten text
	// would match none of those rules, so they are checked here. No allow rule
	// is required; the rewrite carries no permissionDecision, so the harness's
	// permission flow still applies to the rewritten command.
	if blocked == nil || blocked(command) || blocked(d.Argv) {
		return execwrap.Decision{Family: d.Family, Reason: "blocked_by_user_rules"}
	}
	return d
}

// shellRewrite handles a shell-tool call inside runHookAdapter. isShell is
// false when ev is not a shell call, and the spawn path runs as before. When
// isShell is true the call is never routed: out is the rewrite to emit, or nil
// to allow it unchanged.
func shellRewrite[E any](harness string, ev E) (out any, isShell bool) {
	s, ok := any(ev).(shellEvent)
	if !ok {
		return nil, false
	}
	command, isShell := s.ShellCommand()
	if !isShell {
		return nil, false
	}
	d := shellWrapDecision(harness, command, s.ShellBlocked)
	if !d.Rewrite {
		return nil, true
	}
	wrapped := d.Command
	if identified, ok := any(ev).(interface{ SessionIdentifier() string }); ok {
		wrapped = execwrap.WithSession(wrapped, telemetry.HashSessionID(identified.SessionIdentifier()))
	}
	out, err := s.ShellRewrite(wrapped)
	if err != nil {
		return nil, true
	}
	return out, true
}
