// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package classify

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
)

// The two reviewed vocabularies are embedded in the binary. JSON is parsed
// once at package initialization, never in a prompt's routing hot path.
//
//go:embed lexicon/en.json
var englishLexiconJSON []byte

//go:embed lexicon/pt-BR.json
var portugueseLexiconJSON []byte

type lexiconItem struct {
	Order   int    `json:"order"`
	Pattern string `json:"pattern"`
}

type lexicalDictionary struct {
	SchemaVersion int                      `json:"schema_version"`
	Language      string                   `json:"language"`
	Actions       map[string][]lexiconItem `json:"actions"`
	Terms         map[string][]lexiconItem `json:"terms"`
}

type domainLexicon struct {
	actions map[string]string
	terms   map[string]string
}

var (
	lexiconOnce sync.Once
	lexiconData domainLexicon
	lexiconErr  error
)

// mergeLexiconItems restores the original alternation order when Portuguese
// and English aliases are combined. It rejects missing positions, so a
// language split cannot silently drop a synonym or introduce duplicates.
func mergeLexiconItems(items []lexiconItem) (string, error) {
	if len(items) == 0 {
		return "", fmt.Errorf("missing lexical terms")
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Order < items[j].Order })
	out := make([]string, len(items))
	for i, term := range items {
		if term.Order != i {
			return "", fmt.Errorf("non-contiguous or duplicate lexicon position %d at %d", term.Order, i)
		}
		if strings.TrimSpace(term.Pattern) == "" {
			return "", fmt.Errorf("empty lexical pattern at %d", i)
		}
		out[i] = term.Pattern
	}
	return strings.Join(out, "|"), nil
}

func parseDomainLexicon(en, pt []byte) (domainLexicon, error) {
	var left, right lexicalDictionary
	if err := json.Unmarshal(en, &left); err != nil {
		return domainLexicon{}, fmt.Errorf("English lexicon: %w", err)
	}
	if err := json.Unmarshal(pt, &right); err != nil {
		return domainLexicon{}, fmt.Errorf("Portuguese lexicon: %w", err)
	}
	if left.SchemaVersion != 1 || right.SchemaVersion != 1 || left.Language != "en" || right.Language != "pt-BR" {
		return domainLexicon{}, fmt.Errorf("unsupported locale or vocabulary schema")
	}
	out := domainLexicon{actions: make(map[string]string), terms: make(map[string]string)}
	for _, pair := range []struct {
		first, second map[string][]lexiconItem
		result        map[string]string
		name          string
	}{
		{left.Actions, right.Actions, out.actions, "action"},
		{left.Terms, right.Terms, out.terms, "rule"},
	} {
		for key, items := range pair.first {
			pair.result[key] = ""
			_ = items
		}
		for key := range pair.second {
			pair.result[key] = ""
		}
		for key := range pair.result {
			items := append(append([]lexiconItem{}, pair.first[key]...), pair.second[key]...)
			joined, err := mergeLexiconItems(items)
			if err != nil {
				return domainLexicon{}, fmt.Errorf("%s %s: %w", pair.name, key, err)
			}
			if _, err := regexp.Compile("(?i)(?:" + joined + ")"); err != nil {
				return domainLexicon{}, fmt.Errorf("%s %s regex: %w", pair.name, key, err)
			}
			pair.result[key] = joined
		}
	}
	return out, nil
}

// domainPatternFromLexicon produces the exact same action+target RE2 pattern
// that the original Go literals used. Only the source of the vocabulary
// changes, not scoring, confidence, complexity, risk handling or model choice.
func domainPatternFromLexicon(id, action string) string {
	lexiconOnce.Do(func() {
		lexiconData, lexiconErr = parseDomainLexicon(englishLexiconJSON, portugueseLexiconJSON)
	})
	if lexiconErr != nil {
		panic("invalid embedded domain lexicon: " + lexiconErr.Error())
	}
	verbs, foundAction := lexiconData.actions[action]
	targets, foundTerms := lexiconData.terms[id]
	if !foundAction || !foundTerms {
		panic(fmt.Sprintf("missing domain vocabulary: rule=%s action=%s", id, action))
	}
	return domainPattern("(?:"+verbs+")", targets)
}

// domainPattern keeps action and technical target in the same short clause.
// It is intentionally in Go because this is routing logic, not vocabulary.
func domainPattern(action, target string) string {
	return `\b` + action + `\b[^;\n]{0,110}\b(?:` + target + `)\b`
}
