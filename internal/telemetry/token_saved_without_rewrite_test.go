// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/catalog"
	"github.com/tiagovilasboas/downshift/internal/telemetry/catalogtier"
)

// TestTokenDirection_VerdictOKWithoutRewrite (owner decision C3): a verdict OK
// child ran on the model it ran on, so a cheaper tier counts as saved even with
// no applied rewrite. Held and refuted children never rank.
func TestTokenDirection_VerdictOKWithoutRewrite(t *testing.T) {
	cat := catalog.Load()
	prev := DirectionCatalog
	DirectionCatalog = catalogtier.Of(cat)
	defer func() { DirectionCatalog = prev }()
	cases := []struct {
		name, requested, ran, want string
	}{
		{"owner shape: sonnet requested, haiku ran", "claude-sonnet-4-6", "claude-haiku-4-5", TokenCheaper},
		{"same model", "claude-haiku-4-5", "claude-haiku-4-5", TokenUnchanged},
		{"haiku requested, sonnet ran", "claude-haiku-4-5", "claude-sonnet-4-6", TokenPricier},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			usage, decision := rankEvents(t, cat, tc.requested, tc.ran, OutcomeAllow)
			if got := tokenDirection(usage, decision, true, false, false); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
	usage, decision := rankEvents(t, cat, "claude-sonnet-4-6", "claude-haiku-4-5", OutcomeAllow)
	if got := tokenDirection(usage, withCorrections(decision), true, false, false); got != TokenUnchanged {
		t.Fatalf("held verdict OK: got %q, want %q", got, TokenUnchanged)
	}
	if got := tokenDirection(usage, decision, true, false, true); got != TokenUnchanged {
		t.Fatalf("refuted verdict OK: got %q, want %q", got, TokenUnchanged)
	}
}

// TestTokenSavings_SavedWithoutRewriteIsCountedApart: a verdict OK saving folds
// into the saved counts and into saved_without_rewrite; an applied rewrite
// saving does not.
func TestTokenSavings_SavedWithoutRewriteIsCountedApart(t *testing.T) {
	cat := catalog.Load()
	prev := DirectionCatalog
	DirectionCatalog = catalogtier.Of(cat)
	defer func() { DirectionCatalog = prev }()
	okUsage, okDecision := rankEvents(t, cat, "claude-sonnet-4-6", "claude-haiku-4-5", OutcomeAllow)
	okDecision.CorrelationID = "ok"
	okUsage.CorrelationID, okUsage.LinkedDecision = "ok-u", "ok"
	rwUsage, rwDecision := rankEvents(t, cat, "claude-opus-4-8", "claude-haiku-4-5", OutcomeRewriteEmitted)
	rwDecision.CorrelationID = "rw"
	rwUsage.CorrelationID, rwUsage.LinkedDecision = "rw-u", "rw"

	tot := TokenSavings([]Event{okDecision, okUsage, rwDecision, rwUsage}).Total
	if tot.SavedChildren != 2 || tot.WorkSaved != 3000 {
		t.Fatalf("saved children %d work %d, want 2 and 3000", tot.SavedChildren, tot.WorkSaved)
	}
	want := SavedWithoutRewrite{Children: 1, Work: 1500, Cache: 0}
	if tot.SavedWithoutRewrite != want {
		t.Fatalf("saved_without_rewrite = %+v, want %+v", tot.SavedWithoutRewrite, want)
	}
	if line := tot.Line(); !strings.Contains(line, "1 without a rewrite") {
		t.Fatalf("line %q lacks the suffix", line)
	}
	if line := (TokenSaving{}).Line(); strings.Contains(line, "without a rewrite") {
		t.Fatalf("empty line %q has the suffix", line)
	}
}
