// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package playbook holds the working rules a delegated worker receives: named
// markdown sets. General always applies; a delegation may name one more set
// for its area. The person edits them with `downshift playbook`, and the MCP
// delegation server reads them through Block. Rules are instructions to an
// agent, never lint: nothing here enforces them.
//
// Downshift by Tiago Vilas Boas
// https://github.com/tiagovilasboas/downshift
package playbook

import (
	"embed"
	"fmt"
	"strings"
)

// General is the set that always applies. It is shipped and can be edited or
// restored, but never removed.
const General = "general"

// WarnChars is where the CLI starts warning: the text travels with every
// delegation of its area, so its size is a cost the person should see.
const WarnChars = 8000

//go:embed assets/*.md
var assets embed.FS

// Source says where a set came from, for the listing.
type Source string

const (
	SourceDefault Source = "default" // shipped, untouched
	SourceEdited  Source = "edited"  // shipped, changed by the person
	SourceCustom  Source = "custom"  // added by the person
)

// Set is one playbook. Text is the markdown handed to the worker verbatim.
type Set struct {
	ID     string // file stem: "general", "backend", or the slug of a custom name
	Name   string // what a delegation names; unique after folding
	Text   string
	Source Source
}

// Chars is the size of the text in characters (runes), the unit the warning uses.
func (s Set) Chars() int { return len([]rune(s.Text)) }

// shipped reads the default sets from the embedded assets, in display order.
func shipped() []Set {
	order := []string{"general", "backend", "frontend"}
	out := make([]Set, 0, len(order))
	for _, id := range order {
		b, err := assets.ReadFile("assets/" + id + ".md")
		if err != nil {
			panic(fmt.Sprintf("playbook asset %s missing: %v", id, err))
		}
		name, text := splitHeading(string(b))
		out = append(out, Set{ID: id, Name: name, Text: text, Source: SourceDefault})
	}
	return out
}

// splitHeading returns the text of a playbook file after its first "# " line
// and that line's text. A file without a heading is all text, named by the caller.
func splitHeading(body string) (name, text string) {
	lines := strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n")
	i := 0
	for i < len(lines) && strings.TrimSpace(lines[i]) == "" {
		i++
	}
	if i < len(lines) && strings.HasPrefix(lines[i], "# ") {
		name = strings.TrimSpace(strings.TrimPrefix(lines[i], "# "))
		i++
	}
	return name, strings.TrimSpace(strings.Join(lines[i:], "\n"))
}

// fold makes a name match regardless of case and accents, so an agent typing
// "banco de dados" finds "Banco de Dados".
func fold(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(s)) {
		if m, ok := accents[r]; ok {
			r = m
		}
		b.WriteRune(r)
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

var accents = map[rune]rune{
	'á': 'a', 'à': 'a', 'â': 'a', 'ã': 'a', 'ä': 'a',
	'é': 'e', 'è': 'e', 'ê': 'e', 'ë': 'e',
	'í': 'i', 'ì': 'i', 'î': 'i', 'ï': 'i',
	'ó': 'o', 'ò': 'o', 'ô': 'o', 'õ': 'o', 'ö': 'o',
	'ú': 'u', 'ù': 'u', 'û': 'u', 'ü': 'u',
	'ç': 'c', 'ñ': 'n',
}

// Slug turns a name into a file stem: folded, with runs of other characters
// replaced by "-". An empty result means the name has no usable characters.
func Slug(name string) string {
	var b strings.Builder
	dash := false
	for _, r := range fold(name) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			dash = false
			continue
		}
		if !dash && b.Len() > 0 {
			b.WriteByte('-')
			dash = true
		}
	}
	return strings.TrimSuffix(b.String(), "-")
}

// Validate refuses a list whose names collide after folding, or a set with no name.
func Validate(sets []Set) error {
	seen := map[string]string{}
	for _, s := range sets {
		key := fold(s.Name)
		if key == "" {
			return fmt.Errorf("playbook %q has no name", s.ID)
		}
		if other, ok := seen[key]; ok {
			return fmt.Errorf("playbooks %q and %q have the same name", other, s.ID)
		}
		seen[key] = s.ID
	}
	return nil
}

// Find returns the set whose name matches name after folding.
func Find(sets []Set, name string) (Set, bool) {
	key := fold(name)
	for _, s := range sets {
		if fold(s.Name) == key {
			return s, true
		}
	}
	return Set{}, false
}

// Names lists the active names, for error messages and the listing.
func Names(sets []Set) []string {
	out := make([]string, 0, len(sets))
	for _, s := range sets {
		out = append(out, s.Name)
	}
	return out
}

func (s Source) String() string { return string(s) }
