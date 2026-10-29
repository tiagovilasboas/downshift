// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package hint reads what a spawn declares about its own model, as contract
// types only. It is pure: no I/O, no state, no decision.
//
// The parent declares the hint as an enum prefix at the start of the spawn
// description, for example `[kind=code effort=deep tier=small]`
// (docs/design/routing-v2.md sections 2 and 5). The prefix is read
// case-insensitively, and spaces around `=` and between pairs are ignored. A
// key or value that is not a declared enum is ignored, and so is a repeated
// key. A tier above the requested tier is ignored: a hint only moves down or
// keeps. An unknown requested tier keeps the model, so no tier hint applies.
// Doubt keeps the requested model; the zero Hint declares nothing.
//
// The agent type is not a Hint field. Design section 2 matches it against
// profiles and rules through contract.Request.AgentType, so ParseAgentType
// only checks that it is a plain identifier.
//
// Downshift by Tiago Vilas Boas
// https://github.com/tiagovilasboas/downshift
package hint

import (
	"regexp"
	"slices"
	"strings"

	"github.com/tiagovilasboas/downshift/internal/routing/contract"
)

// Keys of the prefix. The spelling is the hint convention; a rename breaks the
// descriptions that already carry it.
const (
	keyKind   = "kind"
	keyEffort = "effort"
	keyTier   = "tier"
)

// agentTypeMaxLen bounds an agent type. Type names are short identifiers
// (docs/design/subagent-type-signal.md).
const agentTypeMaxLen = 64

// spacesAroundEquals joins `kind = code` into `kind=code`.
var spacesAroundEquals = regexp.MustCompile(`\s*=\s*`)

// Parse returns the hint that the description's prefix declares. requested is
// the tier of the model the spawn asked for; a tier hint is kept only when it
// is at or below that tier.
func Parse(description string, requested contract.Tier) contract.Hint {
	var h contract.Hint
	for key, values := range prefixValues(description) {
		if len(values) != 1 {
			continue // a repeated key is ambiguous: ignored
		}
		v := values[0]
		switch key {
		case keyKind:
			if k := contract.Kind(v); k.Valid() {
				h.Kind = k
			}
		case keyEffort:
			if e := contract.EffortClass(v); e.Valid() {
				h.Effort = e
			}
		case keyTier:
			if t := contract.Tier(v); descends(t, requested) {
				h.Tier = t
			}
		}
	}
	return h
}

// ParseAgentType returns the agent type a spawn names, trimmed. It returns ""
// when the name is empty or is not a plain identifier: at most 64 characters
// from A-Z, a-z, 0-9, '.', '_' and '-'. The harness reports the name; it only
// ever matches a declared profile or rule.
func ParseAgentType(raw string) string {
	name := strings.TrimSpace(raw)
	if name == "" || len(name) > agentTypeMaxLen || strings.IndexFunc(name, outsideIdentifier) >= 0 {
		return ""
	}
	return name
}

// prefixValues reads `[key=value ...]` at the start of description into the
// values of each key, lower-cased. A prefix that is not at the start or has no
// closing bracket yields nothing.
func prefixValues(description string) map[string][]string {
	s := strings.TrimSpace(description)
	if !strings.HasPrefix(s, "[") {
		return nil
	}
	body, _, closed := strings.Cut(s[1:], "]")
	if !closed {
		return nil
	}
	values := make(map[string][]string)
	normalized := spacesAroundEquals.ReplaceAllString(strings.ToLower(body), "=")
	for _, field := range strings.Fields(normalized) {
		if key, value, ok := strings.Cut(field, "="); ok {
			values[key] = append(values[key], value)
		}
	}
	return values
}

// descends reports whether tier t is at or below requested. Either tier being
// unknown descends to nothing: doubt keeps the model.
func descends(t, requested contract.Tier) bool {
	if !t.Valid() || !requested.Valid() {
		return false
	}
	tiers := contract.Tiers()
	return slices.Index(tiers, t) <= slices.Index(tiers, requested)
}

// outsideIdentifier reports whether r is outside the agent type alphabet.
func outsideIdentifier(r rune) bool {
	switch {
	case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		return false
	case r == '.', r == '_', r == '-':
		return false
	}
	return true
}
