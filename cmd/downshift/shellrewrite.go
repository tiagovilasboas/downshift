// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os/exec"

	"github.com/tiagovilasboas/downshift/internal/contextopt"
	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/execwrap"
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
	// ShellAllowed reports whether the harness's own permission rules would
	// run the original command without asking: an allow rule matches and no
	// deny or ask rule does. Any rule-reading error is false.
	ShellAllowed(command string) bool
}

// execEnvKillSwitch turns the exec wrapper off everywhere when set to "0".
const execEnvKillSwitch = "DOWNSHIFT_EXEC"

// execConfigEnabled reports whether the exec wrapper is on in the context
// config. Tests swap it.
var execConfigEnabled = func() bool {
	cfg, err := contextopt.LoadConfig()
	return err == nil && cfg.Enabled && cfg.Exec
}

// execLookPath finds the downshift binary the rewritten command will run.
// Tests swap it.
var execLookPath = exec.LookPath

// shellWrapDecision is the shared, harness-free decision for one shell
// command: kill switch, config, policy, the harness's declared capability,
// the user's own permission rules, and a downshift binary on PATH for the
// shell to find. Anything short of all of them leaves the command unchanged.
func shellWrapDecision(harness, command string, allowed func(string) bool) execwrap.Decision {
	if hookGetenv(execEnvKillSwitch) == "0" || !execConfigEnabled() {
		return execwrap.Decision{Reason: "disabled"}
	}
	d := execwrap.Decide(command)
	if !d.Rewrite {
		return d
	}
	if caps, ok := core.CapsFor(harness); !ok || !caps.CanRewriteShellCommand {
		return execwrap.Decision{Family: d.Family, Reason: "harness_cannot_rewrite"}
	}
	// The user's own rules decide: the original command must be allowed and
	// not denied or asked, as written and as normalised. The rewritten text
	// would match none of those rules.
	if allowed == nil || !allowed(command) || !allowed(d.Argv) {
		return execwrap.Decision{Family: d.Family, Reason: "not_allowed_by_user_rules"}
	}
	if _, err := execLookPath("downshift"); err != nil {
		return execwrap.Decision{Family: d.Family, Reason: "downshift_not_on_path"}
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
	d := shellWrapDecision(harness, command, s.ShellAllowed)
	if !d.Rewrite {
		return nil, true
	}
	out, err := s.ShellRewrite(d.Command)
	if err != nil {
		return nil, true
	}
	return out, true
}
