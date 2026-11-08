// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package shellwrap

import (
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/core"
	"github.com/tiagovilasboas/downshift-ai/internal/execwrap"
)

// Compound and chain shapes are wrapped only for a POSIX shell; one plain
// command is wrapped for every dialect.
func TestDecideGatesCompoundAndChainOnShellDialect(t *testing.T) {
	cases := []struct {
		name    string
		dialect string
		command string
		rewrite bool
		reason  string
	}{
		{"posix compound", core.ShellPOSIX, "cd d && go test ./... | tail -5", true, execwrap.ReasonAllowlistedCompound},
		{"posix pipe", core.ShellPOSIX, "go test ./... | tail -5", true, execwrap.ReasonAllowlistedCompound},
		{"posix chain", core.ShellPOSIX, "go version && go vet ./...", true, execwrap.ReasonAllowlistedChain},
		{"posix plain", core.ShellPOSIX, "git status", true, execwrap.ReasonAllowlisted},
		{"unknown compound", "", "cd d && go test ./... | tail -5", false, execwrap.ReasonUnsafeChar},
		{"unknown pipe", "", "go test ./... | tail -5", false, execwrap.ReasonUnsafeChar},
		{"unknown chain", "", "go version && go vet ./...", false, execwrap.ReasonUnsafeChar},
		{"unknown semicolon chain", "", "go version; go vet ./...", false, execwrap.ReasonUnsafeChar},
		{"unknown plain", "", "git status", true, execwrap.ReasonAllowlisted},
		{"other dialect compound", "powershell", "cd d && go test ./... | tail -5", false, execwrap.ReasonUnsafeChar},
		{"other dialect plain", "powershell", "git status", true, execwrap.ReasonAllowlisted},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			withEnv(t, true, nil, true)
			d := decide(c.dialect, c.command, never, true)
			if d.Rewrite != c.rewrite || d.Reason != c.reason {
				t.Fatalf("decide(%q, %q) = rewrite %v reason %q, want %v %q", c.dialect, c.command, d.Rewrite, d.Reason, c.rewrite, c.reason)
			}
		})
	}
}

// Through Handle the dialect comes from the capability table, not from a
// harness id: a declared POSIX harness wraps the compound shape.
func TestHandleWrapsCompoundShapeForDeclaredPOSIXHarness(t *testing.T) {
	withEnv(t, true, nil, true)
	harness := rewritingHarness(t, false, false)
	caps, _ := core.CapsFor(harness)
	if caps.ShellDialect != core.ShellPOSIX {
		t.Fatalf("%s declares dialect %q, want %q", harness, caps.ShellDialect, core.ShellPOSIX)
	}
	r := Handle(harness, fakeShell{command: "cd d && go test ./... | tail -5"}, "")
	if r.Rewrite == nil || r.Decision.Reason != execwrap.ReasonAllowlistedCompound {
		t.Fatalf("compound not wrapped: %+v", r)
	}
}
