// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package classify

import (
	"strings"
	"testing"
)

// neutralContext is ~65 words of project description with no signal, so a
// brief built around it is long (>= minBriefWords) without voting.
const neutralContext = "The repository uses Go 1.27, the standard library and a small set of vetted dependencies. " +
	"The CI pipeline runs on every change and publishes coverage. The maintainers prefer small focused " +
	"commits with clear messages written in English, and every package carries a short doc comment that explains its purpose. " +
	"Several people work in this repository at the same time, each in a separate checkout, so keep the change small and easy to review."

// TestClassify_LongBriefEdgeCases covers long delegation briefs whose context
// mentions hard topics while the actionable task is mechanical, negated scope
// words, and risk words that must keep their floor. Each row states why the
// expectation is correct.
func TestClassify_LongBriefEdgeCases(t *testing.T) {
	tests := []struct {
		name      string
		prompt    string
		want      Complexity
		confident bool
		focus     bool // TaskFocus
		riskFloor bool
		note      string
	}{
		// --- Explicit "Task:" section: the rest is context ---
		{
			name: "explicit task rename, context names architecture and consensus",
			prompt: "Background: " + neutralContext + " The architecture has a classifier, a policy layer and adapters, " +
				"and we reached consensus on the layering rule last week.\n\nTask: rename the variable cnt to count in internal/telemetry/summary.go.",
			want: Trivial, confident: true, focus: true,
			note: "two context mentions count 1 in total; rename (3) wins by 2",
		},
		{
			name: "explicit task typo, distributed raft context",
			prompt: "Context: " + neutralContext + " The service is a distributed key-value store built on raft, " +
				"with multi-region replication owned by another team.\n\nTask: fix a typo in the README heading.",
			want: Trivial, confident: true, focus: true,
			note: "distributed, raft, multi-region and replication describe the project, not the typo",
		},
		{
			name: "explicit task rename, context section after the task",
			prompt: neutralContext + "\n\nTask: rename the helper fmtTok to formatTicket in internal/ui/ticket.go.\n\n" +
				"Background: next quarter we will re-architect the ticket service and migrate it to the new event bus.",
			want: Trivial, confident: true, focus: true,
			note: "a Background section after the task is context; its words count 1",
		},
		{
			name: "explicit task simple change, context mention keeps doubt",
			prompt: "Background: " + neutralContext + " The architecture is a set of services behind one gateway.\n\n" +
				"Task: add a verbose flag to the stats command in cmd/tool/stats.go.",
			want: Complex, confident: false, focus: true,
			note: "add flag (Simple 2) beats the capped mention (1) by only 1: held, never downshifted on doubt",
		},
		{
			name: "explicit task is the hard work",
			prompt: "Context: " + neutralContext + "\n\nTask: rewrite the replication layer to use logical decoding " +
				"and migrate the existing slots to the new format.",
			want: Complex, confident: true, focus: false,
			note: "Complex words inside the task keep full weight",
		},
		{
			name: "requirements after the task stay in the task",
			prompt: "Context: " + neutralContext + "\n\nTask: rename the queue package to jobs.\n" +
				"Requirements: the rename must keep the consensus protocol messages and the sharding keys unchanged.",
			want: Complex, confident: true, focus: false,
			note: "a Requirements section shapes the work; its Complex words vote at full weight",
		},
		{
			name: "review wording in context keeps full weight",
			prompt: "Background: " + neutralContext + " A code review found problems in the importer.\n\n" +
				"Task: rename the importer package to ingest.",
			want: Complex, confident: false, focus: false,
			note: "review and retry wording names the work wherever it appears (briefStrongRE)",
		},

		// --- Risk floors read the whole brief, context included ---
		{
			name: "explicit task rename, auth context",
			prompt: "Context: " + neutralContext + " The auth service issues JWTs and stores refresh tokens; " +
				"the architecture is a monolith.\n\nTask: rename the helper fmtTok to formatToken in internal/auth/tokens.go.",
			want: Medium, confident: false, focus: true, riskFloor: true,
			note: "the task reads Trivial, but auth and secrets in the context floor it to Medium",
		},
		{
			name: "explicit task rename, payments context",
			prompt: "Background: " + neutralContext + " This module handles refunds and payment processing for the store, " +
				"and its architecture is old.\n\nTask: rename the local variable amt to amount in refund.go.",
			want: Medium, confident: false, focus: true, riskFloor: true,
			note: "payment risk anywhere in the brief floors a Trivial task",
		},
		{
			name: "explicit task rename, secrets in a later context section",
			prompt: neutralContext + "\n\nTask: rename the config struct Cfg to Config.\n\n" +
				"Notes: the struct also holds the API keys and the database password loaded at startup.",
			want: Medium, confident: false, focus: false, riskFloor: true,
			note: "secrets in a Notes section still floor the result (the brief already read Trivial without the focus)",
		},

		// --- Imperative sentences, no marker: weaker evidence, nudge kept ---
		{
			name: "imperative rename with an architecture mention stays held",
			prompt: neutralContext + " The architecture is a set of Go services behind a gateway. " +
				"Rename the field amount to amountCents in internal/invoice/invoice.go and its tests.",
			want: Complex, confident: false, focus: true,
			note: "without a marker the length nudge stays: rename 3 vs mention 1 + nudge 1, so doubt holds",
		},
		{
			name: "imperative task with no signal leaves the brief as before",
			prompt: neutralContext + " The architecture is a set of Go services behind a gateway. " +
				"Tidy the error messages in internal/invoice so they start with a lowercase letter.",
			want: Complex, confident: true, focus: false,
			note: "the task carries no signal, so the focus does not engage",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if n := len(strings.Fields(tt.prompt)); n < minBriefWords {
				t.Fatalf("precondition: %d words, want >= %d", n, minBriefWords)
			}
			got := Classify(tt.prompt)
			if got.Complexity != tt.want || got.Confident != tt.confident || got.TaskFocus != tt.focus || got.RiskFloor != tt.riskFloor {
				t.Errorf("got %v confident=%v focus=%v floor=%v scores=%v; want %v confident=%v focus=%v floor=%v (%s)",
					got.Complexity, got.Confident, got.TaskFocus, got.RiskFloor, got.Scores,
					tt.want, tt.confident, tt.focus, tt.riskFloor, tt.note)
			}
		})
	}
}

// TestClassify_ShortBriefExplicitTask covers briefs under minBriefWords. Only
// an explicit "Task:" marker, at a line or sentence start, makes the rest
// context; the guards of the long-brief focus hold unchanged.
func TestClassify_ShortBriefExplicitTask(t *testing.T) {
	tests := []struct {
		name      string
		prompt    string
		want      Complexity
		confident bool
		focus     bool
		riskFloor bool
		note      string
	}{
		{
			name: "inline task rename after a background on one line",
			prompt: "Background: this service is a large monolith with a billing summary, a legacy architecture we will redesign " +
				"next quarter, and many open questions about the migration plan. The team discussed concurrency, caching and the " +
				"overall design at length; none of that is in scope here. Task: rename the variable totalAmt to totalAmount in " +
				"internal/report/summary.go and update its two call sites. Do not change anything else.",
			want: Trivial, confident: true, focus: true,
			note: "architecture, redesign, migration, concurrency and design are context; together they count 1",
		},
		{
			name: "inline task typo after a context sentence",
			prompt: "Context: we are reviewing the whole ingestion pipeline and its distributed design, and a later change will " +
				"rewrite the scheduler. Keep that context in mind but do not act on it. Task: fix the typo 'occured' to " +
				"'occurred' in docs/ingest.md.",
			want: Trivial, confident: true, focus: true,
			note: "distributed and rewrite describe the project; the typo fix is the work",
		},
		{
			name:   "short brief, auth context keeps the risk floor",
			prompt: "Context: the auth service issues JWTs and stores refresh tokens in a legacy architecture. Task: rename fmtTok to formatToken in tokens.go.",
			want:   Medium, confident: false, focus: true, riskFloor: true,
			note: "the task reads Trivial, but auth and secrets in the context floor it to Medium",
		},
		{
			name:   "short brief, the task is the hard work",
			prompt: "Context: the docs are up to date. Task: rewrite the replication layer and migrate the existing slots.",
			want:   Complex, confident: true, focus: false,
			note: "Complex words inside the task keep full weight",
		},
		{
			name:   "short brief, focus never makes Complex confident",
			prompt: "Context: earlier we had to rename half the packages by hand. Task: re-architect the scheduler around a work queue.",
			want:   Complex, confident: false, focus: false,
			note: "without the focus the rename in the context keeps the Complex task unconfident; the focus may not clear that doubt",
		},
		{
			name:   "short prompt without a marker is scored as before",
			prompt: "The architecture is a set of Go services behind a gateway. Rename the field amount to amountCents in invoice.go.",
			want:   Complex, confident: false, focus: false,
			note: "imperative extraction stays a long-brief rule: a short prompt's other sentences may be the work",
		},
		{
			name:   "task word mid-sentence is not a marker",
			prompt: "The architecture is a set of Go services and the task: rename the field amount to amountCents in invoice.go.",
			want:   Complex, confident: false, focus: false,
			note: "only a line or sentence start opens a task section",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if n := len(strings.Fields(tt.prompt)); n >= minBriefWords {
				t.Fatalf("precondition: %d words, want < %d", n, minBriefWords)
			}
			got := Classify(tt.prompt)
			if got.Complexity != tt.want || got.Confident != tt.confident || got.TaskFocus != tt.focus || got.RiskFloor != tt.riskFloor {
				t.Errorf("got %v confident=%v focus=%v floor=%v scores=%v; want %v confident=%v focus=%v floor=%v (%s)",
					got.Complexity, got.Confident, got.TaskFocus, got.RiskFloor, got.Scores,
					tt.want, tt.confident, tt.focus, tt.riskFloor, tt.note)
			}
		})
	}
}

// TestClassify_FixTypoRow: the Trivial typo row takes an article and the
// plural, like the doc-edit verb ("fix the typo", "fix typos").
func TestClassify_FixTypoRow(t *testing.T) {
	for _, p := range []string{
		"fix a typo in the README",
		"fix the typo in the README",
		"Fix the typo 'recieve' in README.md.",
		"fix some typos in docs/setup.md",
		"fix typos in the changelog",
	} {
		t.Run(p, func(t *testing.T) {
			if got := Classify(p); got.Complexity != Trivial || !got.Confident {
				t.Errorf("got %v confident=%v scores=%v; want confident TRIVIAL", got.Complexity, got.Confident, got.Scores)
			}
		})
	}
}

// TestClassify_TaskFocusNeverSpendsOnDoubt checks the guard in focusOnTask: the
// focused tally drops Trivial context votes, which could make a hard task's
// Complex score look confident. It must not.
func TestClassify_TaskFocusNeverSpendsOnDoubt(t *testing.T) {
	p := "Context: " + neutralContext + " Earlier we had to rename half the packages by hand.\n\n" +
		"Task: re-architect the scheduler around a work queue."
	if n := len(strings.Fields(p)); n < minBriefWords {
		t.Fatalf("precondition: %d words", n)
	}
	got := Classify(p)
	if got.Complexity != Complex || got.Confident {
		t.Fatalf("got %v confident=%v scores=%v; want unconfident COMPLEX as without the focus", got.Complexity, got.Confident, got.Scores)
	}
}

// TestClassify_BoundedBriefUntouched: a bounded brief has its own context rule
// (applyBoundedBrief); the task focus does not run on it.
func TestClassify_BoundedBriefUntouched(t *testing.T) {
	p := "Repo /tmp/example (Go). Read AGENTS.md first. " + neutralContext + "\n\nTask: rename cnt to count in stats.go.\n\n" +
		"Edit only stats.go. Run go test ./... before you finish. Reply <=100 words with the diff."
	got := Classify(p)
	if got.TaskFocus {
		t.Fatalf("bounded brief used the task focus: %v scores=%v", got.Complexity, got.Scores)
	}
}

// TestClassify_NegatedScope covers negation of scope words at any prompt
// length, and the phrasings that must not count as a negation.
func TestClassify_NegatedScope(t *testing.T) {
	tests := []struct {
		name      string
		prompt    string
		want      Complexity
		confident bool
		note      string
	}{
		{"do not touch the architecture", "rename foo to bar; do not touch the architecture", Trivial, true,
			"the architecture is scoped out of the work"},
		{"without migrating", "Rename the userId column, without migrating the data", Trivial, true,
			"no migration is part of the task"},
		{"pt-br não mexa na arquitetura", "renomeie foo para bar; não mexa na arquitetura", Trivial, true,
			"PT-BR negation with a change verb"},
		{"pt-br sem migrar", "renomeie a coluna userId sem migrar os dados", Trivial, true,
			"PT-BR negation directly before the scope word"},
		{"don't redesign keeps the migration", "migrate the users table to the new schema; don't redesign the service", Complex, true,
			"only the negated redesign is dropped; the migration still votes"},
		{"negation in another clause", "do not touch the docs; migrate the users table to the new schema", Complex, true,
			"the clause break ends the negation"},
		{"contrast is not a negation", "don't just patch it, redesign the cache layer", Complex, true,
			"a comma and a contrast verb end the negation"},
		{"phenomena are never negated", "fix the bug in the worker pool and make sure it does not deadlock", Complex, false,
			"preventing a deadlock is hard work; deadlock (3) still beats fix bug (2), as before"},
		{"race prevention stays complex", "refactor the cache without introducing a race condition", Complex, false,
			"race condition is a phenomenon, not a scope word: it keeps its vote"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Classify(tt.prompt)
			if got.Complexity != tt.want || got.Confident != tt.confident {
				t.Errorf("got %v confident=%v scores=%v; want %v confident=%v (%s)",
					got.Complexity, got.Confident, got.Scores, tt.want, tt.confident, tt.note)
			}
		})
	}
}

func TestTaskSpans(t *testing.T) {
	tests := []struct {
		name     string
		prompt   string
		explicit bool
		want     []string // task text, trimmed
	}{
		{"marker section to context label", "intro.\n\nTask: rename a to b.\nKeep tests green.\n\nBackground: old notes.",
			true, []string{"rename a to b.\nkeep tests green."}},
		{"markdown heading marker", "## Your task:\nmove the file.\n## Context\nwhy we do it", true, []string{"move the file."}},
		{"pt-br marker", "contexto aqui.\n\n**Tarefa:** renomeie x para y.", true, []string{"renomeie x para y."}},
		{"imperative sentences", "Repo /tmp/x (Go). Read AGENTS.md first. Rename a to b in api.md. Do not commit. Then update the docs.",
			false, []string{"rename a to b in api.md.", "then update the docs."}},
		{"inline marker after a sentence", "background only. task: rename a to b. do not change anything else.",
			true, []string{"rename a to b. do not change anything else."}},
		{"no task found", "This is background only. It has no work verbs.", false, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lower := strings.ToLower(tt.prompt)
			spans, explicit := taskSpans(lower)
			var got []string
			for _, s := range spans {
				got = append(got, strings.TrimSpace(lower[s.start:s.end]))
			}
			if explicit != tt.explicit || strings.Join(got, "|") != strings.Join(tt.want, "|") {
				t.Errorf("got %q explicit=%v; want %q explicit=%v", got, explicit, tt.want, tt.explicit)
			}
		})
	}
}
