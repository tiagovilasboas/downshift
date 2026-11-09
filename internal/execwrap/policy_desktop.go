// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

// DOMAIN: desktop automation. The agent-desktop CLI
// reads the accessibility tree of a desktop app and prints one JSON
// envelope. Only its two tree reads are wrapped: snapshot and find. Both
// write a refmap under agent-desktop's own state directory, which the
// wrapper does not change: it runs the same argv and only re-encodes stdout
// (compressor.FormatAgentDesktop keeps every field).

// FamilyAgentDesktop is `agent-desktop snapshot ...` or `agent-desktop find ...`.
const FamilyAgentDesktop Family = "agent-desktop"

// agentDesktopReads are the subcommands whose output is an accessibility
// tree (snapshot) or a list of tree nodes (find).
var agentDesktopReads = map[string]bool{"snapshot": true, "find": true}

// agentDesktopRefusedFlags add a visual-debug artifact (an HTML file and a
// data.debug block) or print help text instead of an envelope.
var agentDesktopRefusedFlags = []string{"debug", "screenshot", "help", "h", "version", "V"}

// desktopFamily names FamilyAgentDesktop when the subcommand comes first. A
// global flag before it (agent-desktop --session s snapshot) is not parsed:
// such a command runs unwrapped.
func desktopFamily(argv []string) Family {
	if len(argv) < 2 || argv[0] != "agent-desktop" || !agentDesktopReads[argv[1]] {
		return FamilyNone
	}
	if namesFlag(argv[2:], agentDesktopRefusedFlags...) {
		return FamilyNone
	}
	return FamilyAgentDesktop
}
