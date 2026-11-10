// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package shellwrap

import "github.com/tiagovilasboas/downshift-ai/internal/core"

// WouldRewrite reports whether the shell rewrite is on for harness and its
// policy wraps command in `downshift compact` (switches, family, dialect,
// capability and the read-only limit; the user's deny/ask rules are not read
// here). The post-run compaction asks it so a command the pre-run hook
// wrapped is not compacted, nor logged, a second time when the harness reports
// the command as it was written. A harness with no declared capabilities is
// false.
func WouldRewrite(harness, command string) bool {
	caps, ok := core.CapsFor(harness)
	if !ok {
		return false
	}
	d := decide(caps.ShellDialect, command, func(string) bool { return false }, caps.CanRewriteShellCommand)
	return limitToReadOnly(caps.RewriteImpliesApproval || caps.UnknownUserRules, d, command).Rewrite
}
