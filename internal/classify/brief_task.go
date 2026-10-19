// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package classify

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

// A long delegation brief carries context (what the project is, what was
// tried, what must not change) around a short actionable task. Context words
// such as "architecture" or "consensus" describe the project, not the work,
// yet the keyword table scores them as if they were the job. The helpers in
// this file find the task and the negated scope mentions so Classify can
// weigh the context as context. They only ever remove votes.

var (
	// taskMarkerRE opens an explicit task section: "Task:", "## Your task:",
	// "**Tarefa:**", at the start of a line or of a sentence ("... in scope
	// here. Task: rename ..."). The section runs to the next label line or
	// heading.
	taskMarkerRE = regexp.MustCompile(`(?m)(?:^|[.!?][ \t]+)[ \t>*#-]*(?:\*\*)?(?:your\s+|the\s+|sua\s+|a\s+)?(?:task|tarefa)(?:\*\*)?[ \t]*:(?:\*\*)?`)
	// contextLabelRE opens a section that is context, not work: background,
	// notes, history, or how to reply. Requirements, constraints and "done
	// means" sections shape the work and stay in the task.
	contextLabelRE = regexp.MustCompile(`(?m)^[ \t>*-]*(?:#{1,6}[ \t]+)?(?:\*\*)?(?:background|context|contexto|notes?|notas?|history|hist[oó]rico|about|sobre|reply|resposta|responda)\b[^\n:]{0,30}(?:\*\*)?[ \t]*(?::|$)`)
	// imperativeRE matches a sentence that opens with a work verb. Reading,
	// running checks, replying and git side steps are not the work, and a
	// negated sentence ("Do not ...") does not open with a verb, so neither
	// is taken as the task.
	imperativeRE = regexp.MustCompile(`^(?:(?:please|now|then|also|finally|first|next|por\s+favor|agora|depois|ent[aã]o|primeiro|em\s+seguida),?\s+)?(?:` + workVerbs + `)\b`)
	// sentenceLeadRE strips list bullets, numbering and emphasis before a sentence.
	sentenceLeadRE = regexp.MustCompile(`^[\s>*#•\-–—]*(?:\d{1,2}[.)]\s+)?(?:\*\*)?\s*`)
)

const workVerbs = `rename|fix|add|update|move|delete|remove|replace|change|implement|refactor|rewrite|write|create|migrate|redesign|design|build|debug|investigate|find|locate|search|list|review|audit|translate|format|bump|convert|extract|document|explain|generate|port|upgrade|simplify|clean|sort|count|summari[sz]e|analy[sz]e|optimi[sz]e|diagnose|trace|compare|draft|` +
	`renomeie|corrija|conserte|adicione|inclua|atualize|mova|apague|remova|exclua|substitua|troque|altere|mude|implemente|refatore|reescreva|escreva|crie|migre|projete|construa|depure|investigue|encontre|procure|busque|liste|revise|audite|traduza|formate|converta|extraia|documente|explique|gere|simplifique|limpe|ordene|conte|resuma|analise|otimize|diagnostique|compare`

// span is a half-open byte range of the lowercased prompt.
type span struct{ start, end int }

// taskSpans returns the byte ranges of lower that state the actionable task:
// the section after an explicit "Task:" marker when there is one (explicit),
// otherwise every sentence that opens with a work verb. It returns nil when it
// finds neither, and Classify then scores the brief as before.
func taskSpans(lower string) (spans []span, explicit bool) {
	if spans := explicitTaskSpan(lower); spans != nil {
		return spans, true
	}
	for _, s := range sentences(lower) {
		text := lower[s.start:s.end]
		lead := sentenceLeadRE.FindStringIndex(text)
		if imperativeRE.MatchString(text[lead[1]:]) {
			spans = append(spans, s)
		}
	}
	return spans, false
}

// explicitTaskSpan returns the section after an explicit "Task:" marker, or
// nil when the brief has none. Unlike imperative sentences, the marker is the
// author saying the rest is context, so Classify trusts it on a brief of any
// length.
func explicitTaskSpan(lower string) []span {
	loc := taskMarkerRE.FindStringIndex(lower)
	if loc == nil {
		return nil
	}
	start, end := loc[1], len(lower)
	if nl := strings.IndexByte(lower[start:], '\n'); nl >= 0 {
		rest := start + nl + 1
		if next := contextLabelRE.FindStringIndex(lower[rest:]); next != nil {
			end = rest + next[0]
		}
	}
	if strings.TrimSpace(lower[start:end]) == "" {
		return nil
	}
	return []span{{start, end}}
}

// sentences splits lower on line breaks and on ". ", "! ", "? ". A dot inside
// a path or version ("api.md", "v1.2") is not followed by a space and does not
// split.
func sentences(lower string) []span {
	var out []span
	start := 0
	flush := func(end int) {
		if strings.TrimSpace(lower[start:end]) != "" {
			out = append(out, span{start, end})
		}
	}
	for i := 0; i < len(lower); i++ {
		switch c := lower[i]; {
		case c == '\n':
			flush(i)
			start = i + 1
		case (c == '.' || c == '!' || c == '?') && (i+1 == len(lower) || lower[i+1] == ' ' || lower[i+1] == '\t'):
			flush(i + 1)
			start = i + 1
		}
	}
	flush(len(lower))
	return out
}

// inSpans reports whether the match at loc starts inside one of spans.
func inSpans(spans []span, loc []int) bool {
	for _, s := range spans {
		if loc[0] >= s.start && loc[0] < s.end {
			return true
		}
	}
	return false
}

// scopeWordRE is the vocabulary a negation can take out of the work: the
// architecture, a redesign, a rewrite, a migration, the entire codebase. A
// phenomenon (a race condition, a deadlock, a leak) is never negated:
// "make sure it does not deadlock" still asks for hard work.
var scopeWordRE = regexp.MustCompile(`^(?:(?:re-?)?architect|re-?arquitet|arquitet|re-?design|re-?desenh|re-?write|re-?escrev|migr|entire\s|(?:toda|todo)\s)`)

var (
	// negationWords end a negation that may take a scope word out of the work.
	negationWords = map[string]bool{"don't": true, "dont": true, "never": true, "without": true,
		"não": true, "nao": true, "nunca": true, "jamais": true, "sem": true}
	// notAuxiliaries form a negation with a following "not": "do not", "must not".
	notAuxiliaries = map[string]bool{"do": true, "does": true, "must": true, "should": true, "will": true, "need": true}
	// changeVerbs scope a negation to a topic: "do not touch the architecture".
	changeVerbs = map[string]bool{"touch": true, "change": true, "modify": true, "edit": true, "alter": true,
		"rework": true, "redo": true, "mexa": true, "mexer": true, "mude": true, "mudar": true, "altere": true,
		"alterar": true, "toque": true, "tocar": true, "modifique": true, "modificar": true}
	// fillerWords may sit between the negation and the scope word.
	fillerWords = map[string]bool{"the": true, "a": true, "an": true, "any": true, "our": true, "this": true,
		"that": true, "its": true, "existing": true, "current": true, "in": true, "on": true, "to": true, "of": true,
		"o": true, "os": true, "as": true, "na": true, "no": true, "nas": true, "nos": true, "em": true, "de": true,
		"da": true, "do": true, "das": true, "dos": true, "nenhuma": true, "nenhum": true, "qualquer": true,
		"atual": true, "existente": true}
)

// negatedScope reports whether a scope word at loc is negated in its own
// clause: "do not touch the architecture", "don't redesign", "without any
// migration", "não mexa na arquitetura", "sem migrar". The negation must be
// followed by either nothing, or a change verb, before at most three filler
// words; anything else ("don't just patch it, redesign it") is not a negation
// of the scope word.
func negatedScope(lower string, loc []int) bool {
	if !scopeWordRE.MatchString(lower[loc[0]:loc[1]]) {
		return false
	}
	window := lower[max(0, loc[0]-60):loc[0]]
	if i := strings.LastIndexAny(window, ".;:!?,()\n—"); i >= 0 {
		_, size := utf8.DecodeRuneInString(window[i:])
		window = window[i+size:]
	}
	words := strings.Fields(window)
	neg := -1
	for i, w := range words {
		if negationWords[w] || (w == "not" && i > 0 && notAuxiliaries[words[i-1]]) {
			neg = i
		}
	}
	if neg < 0 {
		return false
	}
	rest := words[neg+1:]
	if len(rest) > 0 && changeVerbs[rest[0]] {
		rest = rest[1:]
	}
	if len(rest) > 3 {
		return false
	}
	for _, w := range rest {
		if !fillerWords[w] {
			return false
		}
	}
	return true
}

// dropNegatedScope removes the matches negatedScope reports.
func dropNegatedScope(lower string, locs [][]int) [][]int {
	out := locs[:0:0]
	for _, loc := range locs {
		if !negatedScope(lower, loc) {
			out = append(out, loc)
		}
	}
	return out
}

// anyInSpans reports whether any match starts inside the task.
func anyInSpans(spans []span, locs [][]int) bool {
	for _, loc := range locs {
		if inSpans(spans, loc) {
			return true
		}
	}
	return false
}

// strongMention reports whether a Complex match is review or retry wording,
// which names the kind of work wherever it appears in a brief.
func strongMention(lower string, locs [][]int) bool {
	for _, loc := range locs {
		if briefStrongRE.MatchString(lower[loc[0]:loc[1]]) {
			return true
		}
	}
	return false
}

// spanWords counts the words inside the task spans.
func spanWords(lower string, spans []span) int {
	n := 0
	for _, s := range spans {
		n += len(strings.Fields(lower[s.start:s.end]))
	}
	return n
}
