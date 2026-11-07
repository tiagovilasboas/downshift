// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package shellwrap

import "github.com/tiagovilasboas/downshift/internal/execwrap"

// ReasonBackground is the skip code for a shell call the harness runs in the
// background. The compactor holds output until the command exits, so a
// background command would show nothing while it runs and lose everything if
// it is killed.
const ReasonBackground = "background"

// BackgroundReporter is implemented by adapters whose shell tool can run a
// command in the background. The adapter only reads the payload.
type BackgroundReporter interface {
	// ShellRunsInBackground reports whether this shell call runs in the
	// background (for example a run_in_background field set by the model).
	ShellRunsInBackground() bool
}

// PromptReporter is implemented by adapters whose payload says whether the
// harness will prompt the user before running a tool. The adapter maps its own
// mode names to this answer; the policy on it lives here.
type PromptReporter interface {
	// ShellPromptsSuppressed reports a permission mode in which the harness
	// runs tool calls without asking (an unknown mode counts as one). Then
	// only the harness's deny rules guard the call, and a rule Downshift
	// cannot read would not match the rewritten command.
	ShellPromptsSuppressed() bool
}

// refuseBackground keeps a background shell call as written.
func refuseBackground(ev any, d execwrap.Decision) execwrap.Decision {
	if !d.Rewrite {
		return d
	}
	if b, ok := ev.(BackgroundReporter); ok && b.ShellRunsInBackground() {
		return execwrap.Decision{Family: d.Family, Reason: ReasonBackground}
	}
	return d
}

// promptsSuppressed reports whether the event says the harness will not ask
// the user before running this call. An adapter that does not report it keeps
// its normal prompts.
func promptsSuppressed(ev any) bool {
	p, ok := ev.(PromptReporter)
	return ok && p.ShellPromptsSuppressed()
}
