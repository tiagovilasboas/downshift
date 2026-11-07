// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import (
	"strings"
	"testing"
)

func TestFamilyOfGlab(t *testing.T) {
	for _, command := range []string{
		"glab mr list", "glab mr list --assignee=@me", "glab mr view 482", "glab mr view 482 --comments",
		"glab mr diff 482", "glab mr diff 482 --raw", "glab issue list", "glab issue list --label bug",
		"glab issue view 311", "glab ci status", "glab ci list", "glab pipeline list",
		"glab mr view 482 -R platform/webhooks",
	} {
		if got := FamilyOf(strings.Fields(command)); got != FamilyGlab {
			t.Errorf("FamilyOf(%q) = %q, want %q", command, got, FamilyGlab)
		}
	}
}

// Every other subcommand, every write verb and every flag that opens a
// browser, prints the token or keeps the view live is left as written.
func TestFamilyOfGlabRefusals(t *testing.T) {
	for _, command := range []string{
		// browser, token, live
		"glab mr view 482 -w", "glab mr view 482 --web", "glab mr view --web 482", "glab issue view 311 --web=true",
		"glab mr list --show-token", "glab mr view 482 -cw", "glab ci status --live", "glab ci status -l",
		"glab issue list -l bug", // -l is --live on ci status, so it is refused for every shape
		// sensitive or interactive subcommands
		"glab api projects", "glab api graphql", "glab auth status", "glab auth login", "glab config get token",
		"glab config set token x", "glab variable list", "glab variable get TOKEN", "glab ci trace", "glab ci trace 99",
		"glab ci view", "glab job list", "glab pipeline status", "glab pipeline view", "glab token list",
		// write verbs
		"glab mr create", "glab mr merge 482", "glab mr approve 482", "glab mr update 482", "glab mr close 482",
		"glab mr note 482", "glab mr reopen 482", "glab mr delete 482", "glab mr rebase 482", "glab mr checkout 482",
		"glab issue create", "glab issue close 311", "glab issue note 311", "glab issue update 311", "glab issue delete 311",
		"glab ci run", "glab ci retry 99", "glab ci cancel 99", "glab ci delete 99", "glab pipeline run", "glab pipeline delete 9",
		"glab repo clone x", "glab release create v1", "glab",
		"glab mr", "glab ci", "glab -R x mr list", "glab version",
	} {
		if got := FamilyOf(strings.Fields(command)); got != FamilyNone {
			t.Errorf("FamilyOf(%q) = %q, want none", command, got)
		}
		if d := Decide(command, exe); d.Rewrite && d.Family == FamilyGlab {
			t.Errorf("Decide(%q) wrapped a refused glab shape: %+v", command, d)
		}
	}
}

func TestDecideWrapsReadOnlyGlab(t *testing.T) {
	if d := Decide("glab mr view 482", exe); !d.Rewrite || d.Family != FamilyGlab {
		t.Errorf("Decide(glab mr view 482) = %+v", d)
	}
}

// glab output may carry a token, so the storage port is never used for it.
func TestGateKeeper_GlabGetsNoStore(t *testing.T) {
	keep := func([]byte) (string, bool) { return "id", true }
	if !rawStoreRefused[FamilyGlab] {
		t.Fatal("glab may store raw output")
	}
	if (&gate{keep: keep}).keeper(FamilyGlab) != nil {
		t.Error("glab got a storage port")
	}
}

func TestGlabFamilyCompactsAsGlabFormat(t *testing.T) {
	if !compactibleFamily(FamilyGlab) || commandFormats[FamilyGlab] == "" {
		t.Error("glab has no format")
	}
}
