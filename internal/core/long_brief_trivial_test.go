// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core_test

import (
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/core"
)

func longUnboundedBrief(task string) string {
	return strings.Repeat("context ", 90) + task + " " + strings.Repeat("detail ", 40)
}

func TestLongBrief_TrivialKeywordInBacktickDoesNotVote(t *testing.T) {
	p := longUnboundedBrief("Record `git log --oneline -5` before you change anything. Implement the handler described in docs/tasks/api.md with table-driven tests.")
	if n := len(strings.Fields(p)); n < 80 {
		t.Fatalf("precondition: %d words", n)
	}
	got := core.Classify(p)
	if got.Complexity == core.Trivial && got.Confident {
		t.Fatalf("got confident %v (scores %v), want not confident TRIVIAL", got.Complexity, got.Scores)
	}
}

func TestLongBrief_NegatedGitCommitDoesNotVote(t *testing.T) {
	p := longUnboundedBrief("Do not git commit or push. Implement refresh token rotation in internal/auth/tokens.go as specified in docs/tasks/tokens.md, with tests.")
	got := core.Classify(p)
	if got.Complexity == core.Trivial || got.Complexity == core.Simple {
		t.Fatalf("got %v (scores %v), want at least MEDIUM", got.Complexity, got.Scores)
	}
}

func TestLongBrief_GitBranchLabelDoesNotVote(t *testing.T) {
	p := longUnboundedBrief("Work in the perf worktree (git branch `feature/checkout-perf`, from origin/main). Build the Lighthouse suite described in docs/perf.md and add generated reports to .gitignore if needed.")
	got := core.Classify(p)
	if got.Complexity == core.Trivial && got.Confident {
		t.Fatalf("got confident %v (scores %v)", got.Complexity, got.Scores)
	}
}

func TestLongBrief_ShortRenameStillTrivial(t *testing.T) {
	got := core.Classify("rename the userId variable to user_id")
	if got.Complexity != core.Trivial {
		t.Fatalf("got %v, want TRIVIAL", got.Complexity)
	}
}
