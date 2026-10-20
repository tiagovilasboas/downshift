// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package classify

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync"
)

// The growing domain vocabulary is data, not a parallel classifier. It is
// compiled once and fed through the same scoring, task-focus and risk policy.
// It is opt-in while calibration against reviewed holdout labels is pending.
//
//go:embed lexicon/domains.json
var domainLexiconJSON []byte

const (
	domainLexiconEnv       = "DOWNSHIFT_DOMAIN_LEXICON"
	maxDomainLexiconRules  = 300
	maxDomainPatternLength = 512
)

type domainLexiconDocument struct {
	SchemaVersion int                    `json:"schema_version"`
	Domains       []domainLexiconGroup   `json:"domains"`
}

type domainLexiconGroup struct {
	Domain string                  `json:"domain"`
	Rules  []domainLexiconRule     `json:"rules"`
}

type domainLexiconRule struct {
	ID         string `json:"id"`
	Pattern    string `json:"pattern"`
	Complexity string `json:"complexity"`
	Weight     int    `json:"weight"`
}

// compileDomainLexicon validates the evolving vocabulary before it can vote.
// A missing/invalid domain rule cannot crash a developer's subagent hook.
func compileDomainLexicon(data []byte) ([]signal, error) {
	var doc domainLexiconDocument
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("domain lexicon json: %w", err)
	}
	if doc.SchemaVersion != 1 {
		return nil, fmt.Errorf("unsupported domain lexicon schema %d", doc.SchemaVersion)
	}
	if len(doc.Domains) == 0 {
		return nil, fmt.Errorf("domain lexicon contains no domains")
	}
	seenDomains := make(map[string]bool, len(doc.Domains))
	seenRules := make(map[string]bool)
	out := make([]signal, 0, 64)
	for _, domain := range doc.Domains {
		if domain.Domain == "" || seenDomains[domain.Domain] {
			return nil, fmt.Errorf("missing or duplicate domain %q", domain.Domain)
		}
		seenDomains[domain.Domain] = true
		if len(domain.Rules) == 0 {
			return nil, fmt.Errorf("domain %q contains no rules", domain.Domain)
		}
		for _, rule := range domain.Rules {
			if rule.ID == "" || seenRules[rule.ID] {
				return nil, fmt.Errorf("missing or duplicate rule id %q", rule.ID)
			}
			seenRules[rule.ID] = true
			if len(out) >= maxDomainLexiconRules {
				return nil, fmt.Errorf("domain lexicon exceeds %d rules", maxDomainLexiconRules)
			}
			if rule.Weight < 1 || rule.Weight > 3 {
				return nil, fmt.Errorf("%s: weight must be 1..3", rule.ID)
			}
			if strings.TrimSpace(rule.Pattern) == "" || len(rule.Pattern) > maxDomainPatternLength {
				return nil, fmt.Errorf("%s: empty or oversized regex", rule.ID)
			}
			var cls Complexity
			switch rule.Complexity {
			case "TRIVIAL":
				cls = Trivial
			case "SIMPLE":
				cls = Simple
			case "MEDIUM":
				cls = Medium
			case "COMPLEX":
				cls = Complex
			default:
				return nil, fmt.Errorf("%s: unknown complexity %q", rule.ID, rule.Complexity)
			}
			re, err := regexp.Compile("(?i)" + rule.Pattern)
			if err != nil {
				return nil, fmt.Errorf("%s: invalid regex: %w", rule.ID, err)
			}
			out = append(out, signal{re: re, weight: rule.Weight, class: cls})
		}
	}
	return out, nil
}

var (
	domainCompileOnce sync.Once
	domainSignalSet    []signal
)

// activeSignals keeps the shipping v1 rules unchanged by default. After
// explicit opt-in, append the JSON rules to that same scoring pipeline.
// On invalid JSON, fail open to the known-good classifier; tests catch it.
func activeSignals() []signal {
	if os.Getenv(domainLexiconEnv) != "1" {
		return signals
	}
	domainCompileOnce.Do(func() {
		rules, err := compileDomainLexicon(domainLexiconJSON)
		if err != nil {
			return
		}
		domainSignalSet = make([]signal, 0, len(signals)+len(rules))
		domainSignalSet = append(domainSignalSet, signals...)
		domainSignalSet = append(domainSignalSet, rules...)
	})
	if len(domainSignalSet) == 0 {
		return signals
	}
	return domainSignalSet
}
