// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core_test

import (
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/core"
)

// brief builds a synthetic delegation brief (80+ words) around task, with a
// scope limit, a verification command and a reply budget.
func brief(task string) string {
	return "Repo /tmp/example-service (Go). Read AGENTS.md, docs/layering.md and internal/store/store.go first. " +
		task + " " +
		"Edit only internal/store and its tests; do not touch the README or any other package, and do not run any git command. " +
		"Keep the public API stable and follow the existing error-wrapping style used in the package. " +
		"Run: go test -count=1 ./internal/store/ and go vet ./... before you finish. " +
		"Reply <=150 words: what changed, the tests you added, and the results."
}

func TestBoundedBrief_SingleMentionIsConfidentMedium(t *testing.T) {
	// "graph" names the project, not the work; the brief is bounded.
	p := brief("The service draws a dependency graph in the UI. Implement the Delete method described in docs/tasks/store-delete.md exactly as written, with table-driven tests.")
	if n := len(strings.Fields(p)); n < 80 {
		t.Fatalf("precondition: brief has %d words, want >= 80", n)
	}
	d := core.Route(p, "claude-code", catID("claude-code", core.TierFrontier), cat)
	if d.Complexity != core.Medium || !d.Confident {
		t.Fatalf("got %v confident=%v, want confident MEDIUM", d.Complexity, d.Confident)
	}
	if !checkWrites(d) || d.Tier != core.TierMid {
		t.Fatalf("want an applied downshift to mid, got tier=%v safe=%v corrections=%v", d.Tier, d.SafeVerdict, d.Corrections)
	}
}

func TestBoundedBrief_ConstraintKeywordIsNotTrivial(t *testing.T) {
	// "git commit" and "format" appear as constraints, not as the job.
	p := brief("Do not git commit or push. Keep the current format of log lines. Add retry with backoff to the List method as described in docs/tasks/store-retry.md.")
	got := core.Classify(p)
	if got.Complexity == core.Trivial || got.Complexity == core.Simple {
		t.Fatalf("bounded brief classified %v (scores %v), want at least MEDIUM", got.Complexity, got.Scores)
	}
}

func TestBoundedBrief_TwoComplexRowsStayComplex(t *testing.T) {
	p := brief("Redesign the store so it supports multi-tenant isolation, then migrate the existing rows to the new layout.")
	if got := core.Classify(p); got.Complexity != core.Complex {
		t.Fatalf("got %v (scores %v), want COMPLEX: two corroborating Complex rows", got.Complexity, got.Scores)
	}
}

func TestBoundedBrief_ReviewWordingStaysComplex(t *testing.T) {
	p := brief("Do a code review of internal/store and list concrete defects with file:line.")
	if got := core.Classify(p); got.Complexity != core.Complex {
		t.Fatalf("got %v (scores %v), want COMPLEX: review wording is never discounted", got.Complexity, got.Scores)
	}
}

// The risk rules are untouched: a bounded brief that touches a risk category
// is never downshifted, and a small-tier risky brief keeps the Medium floor.
func TestBoundedBrief_RiskNeverDownshifted(t *testing.T) {
	for _, task := range []string{
		"Implement refresh token rotation in the session handler as described in docs/tasks/tokens.md.",
		"Implement the refund flow for payments processing as described in docs/tasks/refund.md.",
		"Fix the race condition in the cache warmer described in docs/tasks/race.md.",
	} {
		p := brief(task)
		d := core.Route(p, "claude-code", catID("claude-code", core.TierFrontier), cat)
		if len(d.Risk) == 0 {
			t.Fatalf("precondition: %q should touch a risk category", task)
		}
		if checkWrites(d) {
			t.Errorf("risky brief %q was downshifted: tier=%v corrections=%v", task, d.Tier, d.Corrections)
		}
		if d.Tier < core.TierMid {
			t.Errorf("risky brief %q below mid: %v", task, d.Tier)
		}
	}
}

// A long prompt that is not a bounded brief keeps the length nudge.
func TestBoundedBrief_UnboundedLongPromptKeepsNudge(t *testing.T) {
	head := "implement the importer for the new feed"
	short := core.Classify(head)
	long := core.Classify(head + " " + strings.Repeat("context ", 100))
	if long.Scores[core.Complex] != short.Scores[core.Complex]+1 {
		t.Fatalf("long unbounded prompt COMPLEX %d, short %d; want +1", long.Scores[core.Complex], short.Scores[core.Complex])
	}
}
