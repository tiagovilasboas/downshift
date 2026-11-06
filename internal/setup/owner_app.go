// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package setup

import (
	"fmt"
	"path/filepath"
	"strings"
)

// Some MCP hosts own their config file: while the app runs it keeps the file in
// memory and writes it back, dropping an entry setup added from outside. Claude
// Desktop does this (observed on macOS: the downshift entry is gone about three
// minutes after setup). Which target has such an owner app is data
// (Mcp.OwnerApp); this file holds the shared detection and the message
// that setup and doctor both print. Detection is per OS behind build tags
// (owner_app_darwin.go, owner_app_other.go); where it cannot tell, the state is
// unknown and the warning is printed as a note anyway.

// OwnerAppState is whether the app that owns a config file is running.
type OwnerAppState int

const (
	OwnerAppUnknown OwnerAppState = iota
	OwnerAppStopped
	OwnerAppRunning
)

// OwnerAppRunningState is the seam setup and doctor call; tests replace it.
var OwnerAppRunningState = platformOwnerAppState

// processListHas reports whether one line of a process list (one executable
// path per line, as `ps -axo comm=` prints) has name as its base name. The
// match is exact and case-sensitive, so the "Claude" app is not confused with
// the lowercase "claude" CLI.
func processListHas(list, name string) bool {
	for _, line := range strings.Split(list, "\n") {
		line = strings.TrimSpace(line)
		if line != "" && filepath.Base(filepath.FromSlash(line)) == name {
			return true
		}
	}
	return false
}

// OwnerAppFix is the fix text setup and doctor share for a dropped entry.
func OwnerAppFix(app string) string {
	return fmt.Sprintf("quit %s fully (Cmd+Q on macOS, not just close the window), then rerun `downshift setup` from a terminal and reopen %s", app, app)
}

// ownerAppWarning is what setup prints after writing a file the app owns. It
// returns "" when the app is known to be stopped (the write will stick).
func ownerAppWarning(app string, state OwnerAppState) string {
	switch state {
	case OwnerAppStopped:
		return ""
	case OwnerAppRunning:
		return fmt.Sprintf("%s is running and rewrites this file from memory while open, so it will drop the entry within minutes; %s",
			app, OwnerAppFix(app))
	default:
		return fmt.Sprintf("%s rewrites this file from memory while it is open and would drop the entry; if it is running, %s",
			app, OwnerAppFix(app))
	}
}

// ownerAppHold is what setup prints instead of writing a file whose owner app
// is running: the write would be dropped within minutes.
func ownerAppHold(app string) string {
	return fmt.Sprintf("%s is running and rewrites this file from memory while open, so the entry was held (not written); %s",
		app, OwnerAppFix(app))
}

// holdForOwnerApp reports whether setup must not write the MCP file of t
// because the app that owns it is running. It prints the held state (also on
// --dry-run). Where the OS cannot tell (unknown), it returns false and setup
// writes then warns afterwards (setupWarnOwnerApp), as before.
func holdForOwnerApp(o Options, t Target, shown string) bool {
	if t.Mcp == nil || t.Mcp.OwnerApp == "" || OwnerAppRunningState(t.Mcp.OwnerApp) != OwnerAppRunning {
		return false
	}
	if o.Out != nil {
		fmt.Fprintf(o.Out, "%s: MCP server %q held for %s: %s\n\n", t.Name, McpServerName, shown, ownerAppHold(t.Mcp.OwnerApp))
	}
	return true
}
