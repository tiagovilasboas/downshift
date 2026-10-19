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

// A bounded brief is a long delegation prompt that fences its own work. It
// carries at least two of three limits: a scope limit (read-only, edit only X),
// a verification command (go test, npm test, pytest), and a reply budget
// ("reply <= 150 words"). Such a prompt mentions many words ("graph" in the
// project description, "no git commit", "other agents work concurrently") that
// describe context or constraints, not the task.
var (
	briefScopeRE  = regexp.MustCompile(`(?i)\b(read[- ]only|do not (edit|modify|touch)|don't (edit|modify|touch)|edit nothing|edit only|only edit|write only|work only|n[aã]o edite|somente leitura|apenas leitura|stay within these|do not publish, deploy|modify unrelated subsystems|bounded .{0,40} workstream)\b`)
	briefVerifyRE = regexp.MustCompile(`(?i)\b(go (test|vet|build)|npm (run )?(test|build|lint)|pnpm (test|build)|yarn test|pytest|cargo (test|build)|vitest|jest|make (test|check)|run the relevant tests)\b`)
	briefBudgetRE = regexp.MustCompile(`(?i)\b(reply|respond|answer|report|summary|responda)\b[^.\n]{0,40}?(<=|≤|under|at most|no more than|max(imum)?|até)\s*\d+\s*(words|lines|rows|palavras|linhas)|\b(under|<=|≤)\s*\d+\s*(words|palavras)`)
	// Review and retry wording names the kind of work even inside a brief, so
	// it is never discounted (see reviewSignals and the retry rows).
	briefStrongRE = regexp.MustCompile(`(?i)\b(security\s+(audit|review)|code\s+review|trade-?offs?|design\s+doc|rfc|think\s+through|(auditoria|revis\S{1,3}o)\s+de\s+seguran|previous\s+attempt|tentativa\s+anterior|compilation\s+failed|failed\s+to\s+compile|subagent\s+failed|subagente\s+falhou|tests?\s+failed|testes?\s+falharam|fix\s+(the\s+)?(build|compilation))\b`)
	// trivialNegationRE matches immediately before a TRIVIAL keyword on long
	// delegation briefs: "no git commit", "never push", "não rode git pull".
	trivialNegationRE = regexp.MustCompile(`(?i)\b(do not|don't|never|nor|without|not|no|n[aã]o|nunca|sem|jamais)\b[^.;:\n]{0,30}$`)
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
	for _, re := range []*regexp.Regexp{briefScopeRE, briefVerifyRE, briefBudgetRE} {
		if re.MatchString(prompt) {
			n++
		}
	}
	return n >= 2
}

// applyBoundedBrief re-weighs the keyword votes of a bounded brief:
//   - Trivial keywords are constraints or side steps ("do not git commit",
//     "run gofmt"), not a one-step job, so they do not vote.
//   - A single Complex row, unless it is review or retry wording, is a mention
//     and does not vote. Two distinct Complex rows still do; a multi-method
//     implement spec counts as one row.
//   - The brief itself is Medium evidence.
//
// It never touches the risk floor or the risk categories: a risky brief still
// cannot be downshifted (selfcheck R5).
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

func applyBoundedBrief(prompt, lower string, scores map[Complexity]int, implementSpec bool) {
	scores[Trivial] = 0
	rows := 0
	if implementSpec {
		rows++
	}
	for _, s := range signals {
		if s.class == Complex && s.re.MatchString(lower) {
			rows++
		}
	}
	if rows <= 1 && !briefStrongRE.MatchString(prompt) {
		scores[Complex] = 0
	}
	scores[Medium] += briefMediumWeight
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

	// A long brief that states its task gets a second, task-focused tally
	// (focus); focusOnTask decides below which one stands.
	var task []span
	explicitTask := false
	if longBrief && !isBounded {
		task, explicitTask = taskSpans(lower)
	}
	focus := map[Complexity]int{Trivial: 0, Simple: 0, Medium: 0, Complex: 0}
	taskSignal := false
	mentions := 0 // Complex rows that matched only outside the task

	for _, s := range signals {
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
		scores[s.class] += s.weight
		if task == nil {
			continue
		}
		inTask := anyInSpans(task, locs)
		taskSignal = taskSignal || inTask
		switch {
		case s.class == Medium || inTask || (s.class == Complex && strongMention(lower, locs)):
			focus[s.class] += s.weight
		case s.class == Complex:
			mentions++
		}
		// Trivial and Simple words outside the task are side steps and
		// constraints ("commit in English", "add a comment if needed").
	}
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
	// The focused tally gets the length nudge only when the task itself is
	// long; otherwise the brief is long because of its context.
	if taskSignal {
		if spanWords(lower, task) >= minBriefWords {
			focus[Complex]++
		}
		cls = focusOnTask(cls, focus)
	}
	// The risk floor reads the whole brief, context included.
	return ApplyRiskFloor(full, cls)
}

// pickClass returns the top-scoring class and whether it beat the runner-up
// by a clear margin. Ties favour the higher complexity.
func pickClass(scores map[Complexity]int) (Complexity, bool) {
	order := []Complexity{Complex, Medium, Simple, Trivial}
	top := Medium // safe default when nothing matches
	topScore := 0
	for _, c := range order {
		if scores[c] > topScore {
			topScore = scores[c]
			top = c
		}
	}
	second := 0
	for _, c := range order {
		if c == top {
			continue
		}
		if scores[c] > second {
			second = scores[c]
		}
	}
	return top, topScore > 0 && topScore-second >= 2
}

// focusOnTask chooses between the whole-brief result and the task-focused
// tally of a long brief. The focus may lower the class or its confidence; it
// never spends money on doubt:
//   - it never raises the class, and never makes a Complex result confident;
//   - an unconfident lower class only clears confidence on the whole-brief
//     class, so the router keeps the requested model.
func focusOnTask(base Classification, focus map[Complexity]int) Classification {
	top, confident := pickClass(focus)
	if base.UnsupportedLanguage {
		confident = false
	}
	switch {
	case top > base.Complexity,
		top == Complex && confident && !base.Confident,
		top == base.Complexity && confident == base.Confident:
		return base
	case top < base.Complexity && !confident:
		base.Confident = false
		base.TaskFocus = true
		return base
	}
	return Classification{
		Complexity:          top,
		Scores:              focus,
		Confident:           confident,
		UnsupportedLanguage: base.UnsupportedLanguage,
		TaskFocus:           true,
	}
}
