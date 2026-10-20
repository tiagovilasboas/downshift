// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package classify

import (
	"encoding/json"
	"testing"
)

func TestEmbeddedBilingualLexiconCoverage(t *testing.T) {
	lex, err := parseDomainLexicon(englishLexiconJSON, portugueseLexiconJSON)
	if err != nil {
		t.Fatal(err)
	}
	if len(lex.actions) != 9 || len(lex.terms) != 96 {
		t.Fatalf("lexicon: got %d actions and %d rules, want 9 and 96", len(lex.actions), len(lex.terms))
	}
	for _, table := range [][]DomainSignalDef{RawDomainSignals, RawExtendedDomainSignals} {
		for _, r := range table {
			if _, ok := lex.terms[r.ID]; !ok {
				t.Errorf("missing bilingual lexicon for %s", r.ID)
			}
			if r.Pattern == "" {
				t.Errorf("empty compiled pattern for %s", r.ID)
			}
		}
	}
}

func TestBilingualLexiconRejectsInvalidData(t *testing.T) {
	tests := []struct {
		name string
		en   []byte
		pt   []byte
	}{
		{"bad JSON", []byte("{"), portugueseLexiconJSON},
		{"invalid schema", []byte(`{"schema_version":2,"language":"en"}`), portugueseLexiconJSON},
		{"bad locale", []byte(`{"schema_version":1,"language":"pt-BR"}`), portugueseLexiconJSON},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := parseDomainLexicon(tt.en, tt.pt); err == nil {
				t.Fatal("accepted invalid lexicon")
			}
		})
	}
}

func TestBilingualLexiconRejectsDuplicateOrder(t *testing.T) {
	var en lexicalDictionary
	if err := json.Unmarshal(englishLexiconJSON, &en); err != nil {
		t.Fatal(err)
	}
	items := en.Actions["inspect"]
	items = append(items, items[0])
	en.Actions["inspect"] = items
	bad, err := json.Marshal(en)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := parseDomainLexicon(bad, portugueseLexiconJSON); err == nil {
		t.Fatal("duplicate lexicon position was accepted")
	}
}
