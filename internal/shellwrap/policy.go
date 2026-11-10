// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package shellwrap

import (
	"github.com/tiagovilasboas/downshift-ai/internal/core"
	"github.com/tiagovilasboas/downshift-ai/internal/execwrap"
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
	// Skip is the reason code when a shell call was left unwrapped (see
	// execwrap.Skip*); empty when the command was wrapped or advised.
	Skip string
	// SkipMetachar and SkipFamily refine a metachar Skip: the class of the
	// metacharacter and the family named before it. Fixed codes, no text.
	SkipMetachar string
	SkipFamily   string
	// SkipKind is the execwrap.MetacharKind of a metachar Skip, a fixed code.
	SkipKind string
}

// skipped is the Result of a declined wrap, with its skip code and detail.
func skipped(d execwrap.Decision, command string) Result {
	reason := execwrap.SkipReason(d, command)
	class, family := execwrap.SkipDetail(reason, command)
	kind := ""
	if reason == execwrap.SkipMetachar {
		kind = execwrap.MetacharKind(command)
	}
	return Result{IsShell: true, Decision: d, Skip: reason, SkipMetachar: class, SkipFamily: family, SkipKind: kind}
}

// AdviseResult is the sentinel value shellRewrite returns to hookrun when the
// harness uses the advise path (exit-2 + stderr). hookrun reads the Message
// field and writes it to stderr before returning exit 2.
type AdviseResult struct {
	Message string
}

// Denied makes AdviseResult a policy-wire output: the hook exits 2 and the
// message is relayed to the model.
func (a AdviseResult) Denied() (bool, string) { return true, a.Message }

// ReasonNotReadOnly is the skip code for a command left unwrapped because it
// is not one read-only command on a harness whose rewrite answer approves the
// command or whose user rules cannot be read
// (core.HarnessCapabilities.RewriteImpliesApproval, UnknownUserRules), or in a
// permission mode where the harness runs the call without asking
// (PromptReporter).
const ReasonNotReadOnly = "not_read_only"

// limitToReadOnly keeps a rewrite decision only for one read-only command
// when limit is set: the harness would approve the rewritten command, cannot
// show Downshift the user's deny/ask rules, or will not prompt the user. Any
// other command then reaches the harness's own prompt and rules as written.
func limitToReadOnly(limit bool, d execwrap.Decision, command string) execwrap.Decision {
	if d.Rewrite && limit && !execwrap.ReadOnly(command) {
		return execwrap.Decision{Family: d.Family, Reason: ReasonNotReadOnly}
	}
	return d
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
		return handleRewrite(caps, rw, sessionTag, harness)
	}

	// Advise path: exit-2 stderr suggestion (CanAdviseShellWrap).
	if ad, ok := ev.(AdviseAdapter); ok {
		return handleAdvise(caps, ad, sessionTag, harness)
	}

	return Result{}
}

func handleRewrite(caps core.HarnessCapabilities, rw RewriteAdapter, sessionTag, harness string) Result {
	command, isShell := rw.ShellCommand()
	if !isShell {
		return Result{}
	}
	d := decide(caps.ShellDialect, command, rw.ShellBlocked, caps.CanRewriteShellCommand)
	d = refuseBackground(rw, refuseQuotedUnderQuotedRules(rw, d))
	limit := caps.RewriteImpliesApproval || caps.UnknownUserRules || promptsSuppressed(rw)
	d = limitToReadOnly(limit, d, command)
	if !d.Rewrite {
		return skipped(d, command)
	}
	wrapped := d.Command
	if sessionTag != "" {
		wrapped = execwrap.WithSession(wrapped, sessionTag)
	}
	wrapped = execwrap.WithHarness(wrapped, harness)
	out, err := rw.ShellRewrite(wrapped)
	if err != nil {
		return Result{IsShell: true, Decision: d}
	}
	return Result{IsShell: true, Rewrite: out, Decision: d}
}

func handleAdvise(caps core.HarnessCapabilities, ad AdviseAdapter, sessionTag, harness string) Result {
	command, isShell := ad.ShellCommand()
	if !isShell {
		return Result{}
	}
	d := refuseQuotedUnderQuotedRules(ad, decide(caps.ShellDialect, command, ad.ShellBlocked, caps.CanAdviseShellWrap))
	d = limitToReadOnly(promptsSuppressed(ad), refuseBackground(ad, d), command)
	if !d.Rewrite {
		return skipped(d, command)
	}
	if !execwrap.WorthAdvising(d.Family) {
		d = execwrap.Decision{Family: d.Family, Reason: "advise_not_worth_retry"}
		return skipped(d, command)
	}
	wrapped := d.Command
	if sessionTag != "" {
		wrapped = execwrap.WithSession(wrapped, sessionTag)
	}
	wrapped = execwrap.WithHarness(wrapped, harness)
	return Result{IsShell: true, Advise: ad.AdviseMessage(wrapped), Decision: d}
}

// decide applies the shared policy: kill switch, config, execwrap family
// classification, and the harness capability flag. dialect is the harness's
// ShellDialect: compound and chain shapes are wrapped only for a POSIX shell.
// The blocked func carries the harness's own deny/ask rules.
func decide(dialect, command string, blocked func(string) bool, capAllows bool) execwrap.Decision {
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
	d := execwrap.DecideFor(command, exe, dialect == core.ShellPOSIX)
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
