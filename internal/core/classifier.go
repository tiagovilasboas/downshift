// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"regexp"
	"strings"
)

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

// signal is a weighted matcher for a complexity class.
type signal struct {
	re     *regexp.Regexp
	weight int
	class  Complexity
}

// signals are ordered from strongest to weakest evidence. We score every
// class and pick the top; ties break toward the higher complexity to stay
// safe (never underpower a task).
var signals = compileSignals()

var (
	// addStubRE matches one helper added by name. "write" and "implement"
	// stay with the lexicon: implement is already Medium, and a write-spec
	// with no other signal keeps the Medium default.
	addStubRE      = regexp.MustCompile(`(?i)^\s*add\s+[A-Z]\w*\(`)
	exportedCallRE = regexp.MustCompile(`\b[A-Z]\w*\([^)]*\)`)

	// A documentation edit: the prompt opens with an edit verb and its target
	// is prose (README, docs, comments), not code. "fix a typo in the
	// migrations README", "rename cron to scheduler in the docs", "add a graph
	// of weekly usage to the README", and the PT-BR equivalents. Architectural
	// words in such a prompt name the topic of the text, not the work.
	docEditVerbRE = regexp.MustCompile(`(?i)^\W*` + docEditVerb + `\b`)
	// The prose is the target either after a preposition ("in the docs", "ao
	// README") or as the verb's direct object ("update the comment on ...").
	docTargetRE = regexp.MustCompile(`(?i)\b(in|to|into|on|from|inside|within|em|no|na|nos|nas|ao|aos)\s+(the\s+|our\s+|this\s+|o\s+|a\s+)?([\w.-]+\s+){0,2}?` + docNoun + `\b`)
	docObjectRE = regexp.MustCompile(`(?i)^\W*` + docEditVerb + `\s+(the\s+|a\s+|an\s+|our\s+|o\s+|os\s+|as\s+|um\s+|uma\s+)?([\w.-]+\s+)?` + docNoun + `\b`)
	// A second clause may carry real work ("... in the docs and migrate the
	// scheduler"), so only a single short clause is treated as prose-only.
	multiClauseRE = regexp.MustCompile(`(?i)\b(and|then|also|plus|e|depois|tamb\S*m)\b|[;\n]`)
)

const (
	// maxDocEditWords bounds the prose-only damper to short requests.
	maxDocEditWords = 25
	docEditVerb     = `(please\s+|por\s+favor,?\s+)?(rename|fix\s+(a\s+|the\s+|some\s+)?typos?|correct|add|insert|put|update|edit|reword|rephrase|remove|delete|move|mention|renomei\w*|renomear|corrij\w*|corrigir|adicion\w*|acrescent\w*|inclua|incluir|atualiz\w*|remova|remover|insira|inserir|coloque|colocar|mencion\w*)`
	docNoun         = `(readme(\.md)?|docs|documentation|documenta\w*|changelog|docstrings?|comments?|coment\w*|wiki)`
)

// docEditOnly reports whether the prompt is a single short edit whose target
// is documentation or comments. Classify drops Complex keyword hits for it, so
// the word "migration", "cron" or "graph" in a README edit does not buy a
// frontier model. It never raises a class.
func docEditOnly(prompt string, wordCount int) bool {
	return wordCount <= maxDocEditWords &&
		docEditVerbRE.MatchString(prompt) &&
		(docTargetRE.MatchString(prompt) || docObjectRE.MatchString(prompt)) &&
		!multiClauseRE.MatchString(prompt)
}

func compileSignals() []signal {
	out := make([]signal, 0, len(RawSignals))
	for _, r := range RawSignals {
		out = append(out, signal{
			re:     regexp.MustCompile(`(?i)` + r.Pattern),
			weight: r.Weight,
			class:  r.Class,
		})
	}
	return out
}

// Classification is the result of scoring a task prompt.
type Classification struct {
	Complexity Complexity
	Scores     map[Complexity]int
	Confident  bool // true when the top class clearly beat the rest
	// RiskFloor is true when a risk signal lifted a Trivial/Simple result to
	// Medium (see risk.go); Risk lists the matched categories.
	RiskFloor bool
	Risk      []string
	// NoSignal is true when no content signal fired and Medium is only the
	// safe default.
	NoSignal bool
	// SemanticLift is true when the semantic boost raised the complexity above
	// what the regex rules chose. It makes a non-regex lift visible in telemetry.
	SemanticLift bool
}

// Classify scores a task prompt against all complexity classes and returns
// the winning complexity. Deterministic, no LLM call, no network.
//
// Rules:
//   - Sum weights per class from matching signals.
//   - Pick the highest-scoring class.
//   - Tie breaks toward higher complexity (never underpower a task).
//   - No signal at all => Medium (safe default, the harness's usual model).
func Classify(prompt string) Classification {
	lower := strings.ToLower(prompt)
	scores := map[Complexity]int{Trivial: 0, Simple: 0, Medium: 0, Complex: 0}

	for _, s := range signals {
		if s.re.MatchString(lower) {
			scores[s.class] += s.weight
		}
	}

	wordCount := len(strings.Fields(prompt))

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
	if strings.Contains(lower, "implement") && len(exportedCallRE.FindAllString(prompt, -1)) >= 3 {
		scores[Complex] += 2
		contentMatched = true
	}

	// Structural nudge: very short imperative prompts lean trivial — but only
	// to reinforce an existing trivial signal, never to create one.
	if contentMatched && wordCount > 0 && wordCount <= 6 {
		scores[Trivial]++
	}
	// Long, detailed prompts lean toward higher complexity.
	if contentMatched && wordCount >= 80 {
		scores[Complex]++
	}

	// No content signal at all => safe default (the harness's usual model).
	if !contentMatched {
		return Classification{Complexity: Medium, Scores: scores, Confident: false, NoSignal: true}
	}

	// Pick the top score. Iterate high→low so ties favor higher complexity.
	order := []Complexity{Complex, Medium, Simple, Trivial}
	top := Medium // safe default when nothing matches
	topScore := 0
	for _, c := range order {
		if scores[c] > topScore {
			topScore = scores[c]
			top = c
		}
	}

	// Confidence: the top class beat the runner-up by a clear margin.
	second := 0
	for _, c := range order {
		if c == top {
			continue
		}
		if scores[c] > second {
			second = scores[c]
		}
	}
	confident := topScore > 0 && topScore-second >= 2

	return applyRiskFloor(prompt, Classification{
		Complexity: top,
		Scores:     scores,
		Confident:  confident,
	})
}
