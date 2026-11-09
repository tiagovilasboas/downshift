// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import (
	"strings"
	"testing"
)

func TestDesktopFamily(t *testing.T) {
	cases := map[string]Family{
		"agent-desktop snapshot --app Files -i --compact":      FamilyAgentDesktop,
		"agent-desktop snapshot --skeleton":                    FamilyAgentDesktop,
		"agent-desktop snapshot --root @s1:e3":                 FamilyAgentDesktop,
		"agent-desktop find --role button --name Save":         FamilyAgentDesktop,
		"agent-desktop click @s1:e3":                           FamilyNone, // an action
		"agent-desktop type @s1:e4 hi":                         FamilyNone,
		"agent-desktop screenshot":                             FamilyNone,
		"agent-desktop list-windows":                           FamilyNone,
		"agent-desktop --session s1 snapshot":                  FamilyNone, // global flag first
		"agent-desktop snapshot --debug --screenshot out.html": FamilyNone,
		"agent-desktop snapshot --help":                        FamilyNone,
		"agent-desktop find -h":                                FamilyNone,
		"agent-desktop snapshot --screenshot=out.html":         FamilyNone,
		"agent-desktop":                                        FamilyNone,
		"npx agent-desktop snapshot":                           FamilyNone,
	}
	for cmd, want := range cases {
		if got := FamilyOf(strings.Fields(cmd)); got != want {
			t.Errorf("FamilyOf(%q) = %q, want %q", cmd, got, want)
		}
	}
}

func TestDesktopScope(t *testing.T) {
	if DesktopScope("", []string{"agent-desktop", "snapshot"}) != "" {
		t.Error("no session must give no scope")
	}
	a := DesktopScope("s", []string{"agent-desktop", "snapshot", "--app", "Chat"})
	b := DesktopScope("s", []string{"agent-desktop", "snapshot", "--app Chat"})
	if a == "" || a == b {
		t.Errorf("scopes must keep argument boundaries: %q vs %q", a, b)
	}
}
