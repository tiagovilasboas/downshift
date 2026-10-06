// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/core"
)

// stubGraphify points DOWNSHIFT_GRAPHIFY_CMD at a script that reports every
// node as a high-degree hub, so graphify escalates any prompt that names a file.
func stubGraphify(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("shell stub")
	}
	script := filepath.Join(t.TempDir(), "graph.sh")
	body := "#!/bin/sh\ncat >/dev/null\necho '{\"Label\":\"hub\",\"Community\":\"core\",\"Edges\":999,\"Found\":true}'\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DOWNSHIFT_GRAPHIFY", "")
	t.Setenv("DOWNSHIFT_GRAPHIFY_CMD", script)
}

// A graphify escalation is a deterministic signal: an unconfident prompt that
// touches a high-blast-radius file must move off the small model. Route used
// to re-run Classify for Confident, so the escalated upshift was dropped.
func TestRoute_GraphEscalationUpshiftsUnconfidentPrompt(t *testing.T) {
	stubGraphify(t)
	prompt := "make internal/text/strings.go nicer"
	d := core.Route(prompt, "claude-code", "claude-haiku-4-5", cat)
	if !d.GraphEscalated || d.Confident || d.Verdict != core.VerdictUpshift {
		t.Fatalf("precondition: escalated=%v confident=%v verdict=%s complexity=%s",
			d.GraphEscalated, d.Confident, d.Verdict, d.Complexity)
	}
	if !d.ShouldRewriteModel() {
		t.Fatalf("graph-escalated upshift was not rewritten (tier %s, corrections %v)",
			d.Tier, d.Corrections)
	}
}

// Without the escalation the same unconfident prompt stays put.
func TestRoute_UnconfidentPromptWithoutGraphStaysPut(t *testing.T) {
	t.Setenv("DOWNSHIFT_GRAPHIFY_CMD", "")
	d := core.Route("make internal/text/strings.go nicer", "claude-code", "claude-haiku-4-5", cat)
	if d.GraphEscalated || d.Confident {
		t.Fatalf("precondition: escalated=%v confident=%v", d.GraphEscalated, d.Confident)
	}
	if d.Verdict == core.VerdictUpshift && d.ShouldRewriteModel() {
		t.Fatalf("unconfident prompt without graph signal was upshifted to %s", d.Tier)
	}
}

// Graphify only escalates an uncertain classification. A confident one
// (here a clear rename) keeps its class even when the named file is a hub.
func TestRoute_GraphDoesNotEscalateConfidentClassification(t *testing.T) {
	stubGraphify(t)
	prompt := "rename the variable userId to userID in internal/text/strings.go"
	if c := core.ClassifyWithSemantic(prompt); !c.Confident || c.Complexity == core.Complex {
		t.Fatalf("precondition: want a confident non-complex classification, got %+v", c)
	}
	d := core.Route(prompt, "claude-code", "claude-haiku-4-5", cat)
	if d.GraphEscalated || d.Complexity == core.Complex {
		t.Fatalf("confident classification escalated: escalated=%v complexity=%s", d.GraphEscalated, d.Complexity)
	}
}
