// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package classify

import "regexp"

var (
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
