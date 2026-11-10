// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package classify

import (
	"strings"

	"github.com/tiagovilasboas/downshift-ai/internal/lazyre"
)

// A bounded brief is a long delegation prompt that fences its own work. It
// carries at least two of three limits: a scope limit (read-only, edit only X),
// a verification command (go test, npm test, pytest), and a reply budget
// ("reply <= 150 words"). Such a prompt mentions many words ("graph" in the
// project description, "no git commit", "other agents work concurrently") that
// describe context or constraints, not the task.
var (
	briefScopeRE  = lazyre.New(`(?i)\b(read[- ]only|do not (edit|modify|touch)|don't (edit|modify|touch)|edit nothing|edit only|only edit|write only|work only|n[aã]o edite|somente leitura|apenas leitura|stay within these|do not publish, deploy|modify unrelated subsystems|bounded .{0,40} workstream)\b`)
	briefVerifyRE = lazyre.New(`(?i)\b(go (test|vet|build)|npm (run )?(test|build|lint)|pnpm (test|build)|yarn test|pytest|cargo (test|build)|vitest|jest|make (test|check)|run the relevant tests)\b`)
	briefBudgetRE = lazyre.New(`(?i)\b(reply|respond|answer|report|summary|responda)\b[^.\n]{0,40}?(<=|≤|under|at most|no more than|max(imum)?|até)\s*\d+\s*(words|lines|rows|palavras|linhas)|\b(under|<=|≤)\s*\d+\s*(words|palavras)`)
	// Review and retry wording names the kind of work even inside a brief, so
	// it is never discounted (see reviewSignals and the retry rows).
	briefStrongRE = lazyre.New(`(?i)\b(security\s+(audit|review)|code\s+review|trade-?offs?|design\s+doc|rfc|think\s+through|(auditoria|revis\S{1,3}o)\s+de\s+seguran|previous\s+attempt|tentativa\s+anterior|compilation\s+failed|failed\s+to\s+compile|subagent\s+failed|subagente\s+falhou|tests?\s+failed|testes?\s+falharam|fix\s+(the\s+)?(build|compilation))\b`)
	// trivialNegationRE matches immediately before a TRIVIAL keyword on long
	// delegation briefs: "no git commit", "never push", "não rode git pull".
	trivialNegationRE = lazyre.New(`(?i)\b(do not|don't|never|nor|without|not|no|n[aã]o|nunca|sem|jamais)\b[^.;:\n]{0,30}$`)
)

const (
	// minBriefWords is where a prompt reads as a delegation brief, not a request.
	minBriefWords = 80
	// briefMediumWeight is the Medium evidence a bounded brief carries: enough
	// to beat one keyword mention with a clear margin, not two corroborating
	// Complex rows.
	briefMediumWeight = 2
)

// boundedBrief reports whether the prompt is a long brief with at least two
// of the three limits above.
func boundedBrief(prompt string, wordCount int) bool {
	if wordCount < minBriefWords {
		return false
	}
	n := 0
	for _, re := range []*lazyre.Regexp{briefScopeRE, briefVerifyRE, briefBudgetRE} {
		if re.MatchString(prompt) {
			n++
		}
	}
	return n >= 2
}

// trivialInCommandQuote reports whether byte offset start lies inside an inline
// command or path quoted with backticks.
func trivialInCommandQuote(prompt string, start int) bool {
	if start <= 0 || start > len(prompt) {
		return false
	}
	return strings.Count(prompt[:start], "`")%2 == 1
}

// trivialInNegation reports whether a TRIVIAL keyword at start is negated or
// forbidden in the words immediately before it ("do not format", "no git commit").
func trivialInNegation(lower string, start int) bool {
	if start <= 0 {
		return false
	}
	window := lower[max(0, start-45):start]
	return trivialNegationRE.MatchString(window)
}

// trivialJobLocs keeps the TRIVIAL matches that are not a constraint mention
// (negation, backtick command quote, git branch label, path segment). A long
// brief's Trivial row votes, once, only when one of them is left.
func trivialJobLocs(prompt, lower string, locs [][]int) [][]int {
	var out [][]int
	for _, loc := range locs {
		if trivialInCommandQuote(prompt, loc[0]) || trivialInNegation(lower, loc[0]) ||
			trivialGitBranchMention(lower, loc[0]) || trivialRenameInPath(lower, loc[0]) {
			continue
		}
		out = append(out, loc)
	}
	return out
}

// trivialGitBranchMention is a repo label ("git branch `feature/foo`"), not a
// one-step git command.
func trivialGitBranchMention(lower string, start int) bool {
	return strings.HasPrefix(lower[start:], "git branch")
}

// trivialRenameInPath matches "mech-rename" path segments, not a rename task.
func trivialRenameInPath(lower string, start int) bool {
	if start == 0 {
		return false
	}
	switch lower[start-1] {
	case '-', '/', '.', '_':
		return true
	default:
		return false
	}
}

// applyBoundedBrief re-weighs the keyword votes of a bounded brief:
//   - Trivial keywords are constraints or side steps ("do not git commit",
//     "run gofmt"), not a one-step job, so they do not vote.
//   - A single Complex row, unless it is review or retry wording, is a mention
//     and does not vote. Two distinct Complex rows still do; a multi-method
//     implement spec counts as one row.
//   - The brief itself is Medium evidence.
//
// It never touches the risk floor or the risk categories: a risky brief keeps
// its categories, so the class step (routing/decide classChoice) never moves it
// down.
func applyBoundedBrief(prompt, lower string, scores map[Complexity]int, implementSpec bool) {
	scores[Trivial] = 0
	rows := 0
	if implementSpec {
		rows++
	}
	seenDomains := map[string]bool{}
	for _, s := range activeSignals() {
		if s.class != Complex || !s.re.MatchString(lower) {
			continue
		}
		if s.domain != "" && osDomainDedupEnabled() {
			if seenDomains[s.domain] {
				continue
			}
			seenDomains[s.domain] = true
		}
		rows++
	}
	if rows <= 1 && !briefStrongRE.MatchString(prompt) {
		scores[Complex] = 0
	}
	scores[Medium] += briefMediumWeight
}
