// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package classify scores a task prompt into a complexity class.
// It does not choose a model tier. Route policy in internal/core does that.
package classify

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

// maxClassifyBytes caps the text the regex table, the risk floor and the
// structural rules look at. Classification cost is linear in prompt size times
// about 100 patterns, and the hook reads up to 1 MiB. A signal past the cap is
// ignored by the regex table by design; the word count and the risk floor
// still read the full prompt, so a risk signal past the cap is not lost.
const maxClassifyBytes = 32 << 10

// classifyHead returns at most maxClassifyBytes of prompt, cut on a rune
// boundary so a multi-byte character is never split.
func classifyHead(prompt string) string {
	if len(prompt) <= maxClassifyBytes {
		return prompt
	}
	cut := maxClassifyBytes
	for cut > 0 && !utf8.RuneStart(prompt[cut]) {
		cut--
	}
	return prompt[:cut]
}

// Complexity is the difficulty class of a task. It maps directly to a Tier.
type Complexity int

const (
	// Trivial — mechanical work: rename, format, move, git ops, boilerplate.
	Trivial Complexity = iota
	// Simple — one isolated change: a field, a function, a small bug fix.
	Simple
	// Medium — a feature across a few files, a refactor with context.
	Medium
	// Complex — architecture, cross-system debugging, security, migrations.
	Complex
)

// String returns the human-readable complexity name.
func (c Complexity) String() string {
	switch c {
	case Trivial:
		return "TRIVIAL"
	case Simple:
		return "SIMPLE"
	case Medium:
		return "MEDIUM"
	case Complex:
		return "COMPLEX"
	default:
		return "UNKNOWN"
	}
}

var (
	// addStubRE matches one helper added by name. "write" and "implement"
	// stay with the lexicon: implement is already Medium, and a write-spec
	// with no other signal keeps the Medium default.
	addStubRE      = regexp.MustCompile(`(?i)^\s*add\s+[A-Z]\w*\(`)
	exportedCallRE = regexp.MustCompile(`\b[A-Z]\w*\([^)]*\)`)
)

// Classification is the result of scoring a task prompt.
type Classification struct {
	Complexity Complexity
	Scores     map[Complexity]int
	Confident  bool // true when the top class clearly beat the rest
	// RiskFloor is true when a risk signal lifted a Trivial/Simple result to
	// Medium (see risk.go); Risk lists the matched categories.
	RiskFloor bool
	// RiskAbstain is true when a risk rule cleared Confident at Medium/Simple
	// without changing complexity (H14 delegation / parallel buckets).
	RiskAbstain bool
	Risk        []string
	// NoSignal is true when no content signal fired and Medium is only the
	// safe default.
	NoSignal bool
	// SemanticLift is true when the semantic boost raised the complexity above
	// what the regex rules chose. It makes a non-regex lift visible in telemetry.
	SemanticLift bool
	// UnsupportedLanguage is true when the prompt reads as a language the
	// tables do not cover (lang_guard.go). Confident is then false.
	UnsupportedLanguage bool
	// TaskFocus is true when the actionable task of a long brief, scored with
	// the rest of the brief as context (focusOnTask), lowered the class or
	// changed its confidence.
	TaskFocus bool
}

// Classify scores a task prompt against all complexity classes and returns
// the winning complexity. Deterministic, no LLM call, no network.
//
// Rules:
//   - Sum weights per class from matching signals.
//   - Pick the highest-scoring class.
//   - Tie breaks toward higher complexity (never underpower a task).
//   - No signal at all => Medium (safe default, the harness's usual model).
//   - Only the first 32 KB is scored (see maxClassifyBytes); word-count rules
//     use the full prompt.
func Classify(prompt string) Classification {
	wordCount := len(strings.Fields(prompt)) // full text, before the cap
	full := prompt
	// Like the risk floor, language coverage must include text past the scoring
	// cap. A foreign-language tail may contain risk the rule table cannot read.
	foreign, _ := languageGuardState(full)
	prompt = classifyHead(prompt)
	lower := strings.ToLower(prompt)
	scores := map[Complexity]int{Trivial: 0, Simple: 0, Medium: 0, Complex: 0}

	longBrief := wordCount >= minBriefWords
	isBounded := longBrief && boundedBrief(prompt, wordCount)

	// A brief that states its task gets a second, task-focused tally (focus);
	// focusOnTask decides below which one stands. Below minBriefWords only an
	// explicit "Task:" marker counts: a short prompt's other sentences may
	// still be the work.
	var task []span
	explicitTask := false
	switch {
	case isBounded:
		// A bounded brief has its own context rule (applyBoundedBrief).
	case longBrief:
		task, explicitTask = taskSpans(lower)
	default:
		task = explicitTaskSpan(lower)
		explicitTask = task != nil
	}
	focus := map[Complexity]int{Trivial: 0, Simple: 0, Medium: 0, Complex: 0}
	var domainScores, domainFocus domainEvidence
	taskSignal := false
	mentions := 0 // Complex rows that matched only outside the task
	mentionDomains := map[string]bool{}

	for _, s := range activeSignals() {
		if !s.re.MatchString(lower) {
			continue
		}
		locs := s.re.FindAllStringIndex(lower, -1)
		if s.class == Complex {
			// "do not touch the architecture" takes the architecture out of the work.
			locs = dropNegatedScope(lower, locs)
		}
		if longBrief && !isBounded && s.class == Trivial {
			locs = trivialJobLocs(prompt, lower, locs)
		}
		if len(locs) == 0 {
			continue
		}
		if s.domain == "" {
			scores[s.class] += s.weight
		} else {
			domainScores.add(s.domain, s.class, s.weight)
		}
		if task == nil {
			continue
		}
		inTask := anyInSpans(task, locs)
		taskSignal = taskSignal || inTask
		switch {
		case s.class == Medium || inTask || (s.class == Complex && strongMention(lower, locs)):
			if s.domain == "" {
				focus[s.class] += s.weight
			} else {
				domainFocus.add(s.domain, s.class, s.weight)
			}
		case s.class == Complex:
			if s.domain == "" || !osDomainDedupEnabled() || !mentionDomains[s.domain] {
				mentions++
			}
			if s.domain != "" {
				mentionDomains[s.domain] = true
			}
		}
		// Trivial and Simple words outside the task are side steps and
		// constraints ("commit in English", "add a comment if needed").
	}
	// Domain matches are pooled by specialty and complexity; generic matches
	// keep the original additive scoring and risk signals are not touched.
	domainScores.apply(scores)
	domainFocus.apply(focus)

	// Outside the task a Complex word describes the project, not the work.
	// Each such row counts 1. Under an explicit "Task:" section the author has
	// said the rest is context, so all of them together count 1.
	if explicitTask {
		mentions = min(mentions, 1)
	}
	focus[Complex] += mentions

	// Architectural vocabulary inside a prose-only edit is the topic of the
	// text, not the size of the work (see docEditOnly).
	if docEditOnly(prompt, wordCount) {
		scores[Complex] = 0
	}

	// Track whether any content signal fired. Structural nudges alone must not
	// decide the class — a vague prompt with no real signal stays Medium.
	contentMatched := scores[Trivial]+scores[Simple]+scores[Medium]+scores[Complex] > 0

	// A silent "add Name(...)" stub is one helper. Apply it only when nothing
	// else matched, and only for a single call, so a write-spec or a multi-step
	// task is not pushed onto the small tier.
	if !contentMatched && addStubRE.MatchString(prompt) && wordCount > 0 && wordCount <= 25 &&
		len(exportedCallRE.FindAllString(prompt, -1)) == 1 {
		scores[Simple] += 2
		contentMatched = true
	}
	// Multi-method implement specs (NewX/Get/Put style) need frontier torque.
	implementSpec := strings.Contains(lower, "implement") && len(exportedCallRE.FindAllString(prompt, -1)) >= 3
	if implementSpec {
		scores[Complex] += 2
		focus[Complex] += 2
		contentMatched = true
	}

	// Structural nudge: very short imperative prompts lean trivial — but only
	// to reinforce an existing trivial signal, never to create one. The point
	// is withheld when Complex is already at least as strong, so a six-word
	// Portuguese twin cannot turn a Complex tie into Trivial. Withholding it
	// whenever Trivial is not strictly ahead would also drop the point that
	// keeps a short unsignalled helper unconfident (R6).
	if contentMatched && wordCount > 0 && wordCount <= 6 && (scores[Complex] == 0 || scores[Trivial] > scores[Complex]) {
		scores[Trivial]++
	}
	// A bounded delegation brief is long because it lists files, limits and
	// checks, not because the work is hard: it gets no length nudge and weighs
	// its keywords as context (applyBoundedBrief). On real subagent briefs the
	// length nudge alone decided most COMPLEX results. Other long, detailed
	// prompts still lean toward higher complexity.
	if contentMatched && isBounded {
		applyBoundedBrief(prompt, lower, scores, implementSpec)
	} else if contentMatched && longBrief {
		scores[Complex]++
	}

	// No content signal at all => safe default (the harness's usual model).
	if !contentMatched {
		return Classification{Complexity: Medium, Scores: scores, Confident: false, NoSignal: true, UnsupportedLanguage: foreign}
	}

	top, confident := pickClass(scores)
	// Outside English and Portuguese a risk word is invisible to risk.go, so
	// a keyword hit is not enough to act on (see lang_guard.go).
	if foreign {
		confident = false
	}
	cls := Classification{
		Complexity:          top,
		Scores:              scores,
		Confident:           confident,
		UnsupportedLanguage: foreign,
	}
	// Only an explicit "Task:" section says the rest of the brief is context,
	// so only then does the focused tally drop the length nudge (unless the
	// task itself is long). Sentences around imperative ones may describe
	// the work itself.
	if taskSignal {
		if !explicitTask || spanWords(lower, task) >= minBriefWords {
			focus[Complex]++
		}
		cls = focusOnTask(cls, focus)
	}
	// The risk floor reads the whole brief, context included.
	return ApplyRiskFloor(full, cls)
}
