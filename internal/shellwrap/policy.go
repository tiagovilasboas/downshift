// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package shellwrap

import (
	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/execwrap"
)

// Result is what Handle returns to the cmd/ orchestrator. The orchestrator
// acts on it without knowing which path was taken.
type Result struct {
	// IsShell is true when the event is a shell-tool call (regardless of
	// whether the command is wrapped). The caller must not route shell calls.
	IsShell bool
	// Rewrite is the wire output to emit when the command should be replaced
	// in place. Nil means allow unchanged.
	Rewrite any
	// Advise is the stderr message for exit-2 advise mode. Empty means allow.
	Advise string
	// Decision carries the family and reason for telemetry.
	Decision execwrap.Decision
}

// AdviseResult is the sentinel value shellRewrite returns to hookrun when the
// harness uses the advise path (exit-2 + stderr). hookrun reads the Message
// field and writes it to stderr before returning exit 2.
type AdviseResult struct {
	Message string
}

// configEnabled and executable are vars so tests in cmd/ can swap them.
// They are set by the cmd/ orchestrator via SetEnvFuncs before any call.
var configEnabled func() bool
var resolveExecutable func() (string, error)
var getenv func(string) string

// SetEnvFuncs wires the three environment accessors that the policy reads.
// cmd/ calls this once at startup with the same funcs it uses today, so the
// package has no direct dependency on os.Getenv or filesystem paths.
func SetEnvFuncs(
	isEnabled func() bool,
	executable func() (string, error),
	env func(string) string,
) {
	configEnabled = isEnabled
	resolveExecutable = executable
	getenv = env
}

// Handle is the single entry point for the shell-wrap domain.
//
//   - If ev does not implement RewriteAdapter or AdviseAdapter: Result{}.
//   - If it is a shell call but the policy says allow: Result{IsShell: true}.
//   - If policy says rewrite (RewriteAdapter):      Result{IsShell:true, Rewrite:<wire>}.
//   - If policy says advise (AdviseAdapter):        Result{IsShell:true, Advise:<msg>}.
func Handle(harness string, ev any, sessionTag string) Result {
	caps, ok := core.CapsFor(harness)
	if !ok {
		return Result{}
	}

	// Rewrite path: in-place command replacement (CanRewriteShellCommand).
	if rw, ok := ev.(RewriteAdapter); ok {
		return handleRewrite(harness, caps, rw, sessionTag)
	}

	// Advise path: exit-2 stderr suggestion (CanAdviseShellWrap).
	if ad, ok := ev.(AdviseAdapter); ok {
		return handleAdvise(harness, caps, ad, sessionTag)
	}

	return Result{}
}

func handleRewrite(harness string, caps core.HarnessCapabilities, rw RewriteAdapter, sessionTag string) Result {
	command, isShell := rw.ShellCommand()
	if !isShell {
		return Result{}
	}
	d := decide(harness, command, rw.ShellBlocked, caps.CanRewriteShellCommand)
	if !d.Rewrite {
		return Result{IsShell: true, Decision: d}
	}
	wrapped := d.Command
	if sessionTag != "" {
		wrapped = execwrap.WithSession(wrapped, sessionTag)
	}
	out, err := rw.ShellRewrite(wrapped)
	if err != nil {
		return Result{IsShell: true, Decision: d}
	}
	return Result{IsShell: true, Rewrite: out, Decision: d}
}

func handleAdvise(harness string, caps core.HarnessCapabilities, ad AdviseAdapter, sessionTag string) Result {
	command, isShell := ad.ShellCommand()
	if !isShell {
		return Result{}
	}
	d := decide(harness, command, ad.ShellBlocked, caps.CanAdviseShellWrap)
	if !d.Rewrite {
		return Result{IsShell: true, Decision: d}
	}
	wrapped := d.Command
	if sessionTag != "" {
		wrapped = execwrap.WithSession(wrapped, sessionTag)
	}
	return Result{IsShell: true, Advise: ad.AdviseMessage(wrapped), Decision: d}
}

// decide applies the shared policy: kill switch, config, execwrap family
// classification, and the harness capability flag. The blocked func carries
// the harness's own deny/ask rules.
func decide(harness, command string, blocked func(string) bool, capAllows bool) execwrap.Decision {
	if getenv == nil || getenv(execwrap.KillSwitchEnv) == "0" ||
		getenv(execwrap.DepthEnv) != "" ||
		(configEnabled != nil && !configEnabled()) {
		return execwrap.Decision{Reason: "disabled"}
	}
	if resolveExecutable == nil {
		return execwrap.Decision{Reason: "executable_unavailable"}
	}
	exe, err := resolveExecutable()
	if err != nil {
		return execwrap.Decision{Reason: "executable_unavailable"}
	}
	d := execwrap.Decide(command, exe)
	if !d.Rewrite {
		return d
	}
	if !capAllows {
		return execwrap.Decision{Family: d.Family, Reason: "harness_cannot_rewrite"}
	}
	if blocked == nil || blocked(command) || blocked(d.Argv) || anySegmentBlocked(blocked, d.Segments) {
		return execwrap.Decision{Family: d.Family, Reason: "blocked_by_user_rules"}
	}
	return d
}

func anySegmentBlocked(blocked func(string) bool, segments []string) bool {
	for _, s := range segments {
		if blocked(s) {
			return true
		}
	}
	return false
}
