// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: BUSL-1.1
// Commercial use requires a licence — see LICENSE for terms.

package core

import (
	"regexp"
	"sort"
	"strings"
)

// RiskSignalDef is one deterministic risk matcher. Risk signals do not vote
// for a complexity class; they set a floor. A task that touches secrets,
// auth, crypto, money, PII, destructive operations, or carries an
// instruction-override phrase must never be routed to the small tier on the
// strength of a single "rename"/"typo"/"format" keyword.
type RiskSignalDef struct {
	Category string
	Pattern  string
}

// RawRiskSignals is the canonical risk table. Patterns describe what the
// task touches (general security/data-safety vocabulary), not phrasings of
// any benchmark prompt. Money is matched on behaviour (charging, refunds,
// settlement, payment processing), not on data shapes that merely name it
// (a billingAddress field). To extend it, add a row and a table-driven case
// in risk_test.go.
var RawRiskSignals = []RiskSignalDef{
	{"secret", `\b(secrets?|passwords?|passphrases?|credentials?|api[ _-]?keys?|private[ _-]?keys?|signing[ _-]?keys?|(access|refresh|bearer|session|auth) tokens?|tokens? validation|jwts?|oauth\w*|saml|sso|2fa|mfa|totp)\b`},
	{"authz", `\b(permissions?|privileges?|is[ _]?admin|admin (role|rights|access)|rbac|acls?|authori[sz]ation|authenticat\w*|access control)\b`},
	// "signature" alone is not crypto ("function signature", "type signature",
	// "email signature"); only verification and signed-message contexts are.
	{"crypto", `\b(crypto\w*|encrypt\w*|decrypt\w*|hmac|tls|ssl|certificates?|(webhook|request|payload|message|digital|cryptographic|jwt|jws|hmac|rsa|ecdsa|ed25519|gpg|pgp) signatures?|signatures? (verification|validation|check|header|secret)s?|(verify|verifies|verifying|validate|validating|check|checking|skip|skipping) (the |a |an |its )?signatures?|signing (secrets?|certificates?))\b`},
	{"payment", `\b(payments? (processing|logic|flow|handler|service|gateway|capture)|process(ing)? payments?|charg(e|es|ing) (the )?(customer|card|user)s?|double[- ]charg\w*|refunds?|settlements?|payouts?|stripe|pci|billing (logic|flow|cycle|calculation|engine))\b`},
	{"vuln", `\b(sql injection|injection|xss|csrf|ssrf|rce|sanitiz\w*|vulnerab\w*|cve-\d+|exploit\w*|security)\b`},
	{"pii", `\b(pii|gdpr|lgpd|personal data|ssn|social security|credit cards?)\b`},
	{"destructive", `rm\s+-rf|\bdrop\s+(the\s+)?(table|database|schema|column)s?\b|\btruncate\s+(the\s+)?table\b|\bdelete\s+(all|every)\b|\bforce[- ]push\b|push\s+(-f|--force)\b|reset\s+--hard|\bwipe\b|\bpurge\b|\bprod(uction)? (data|database|db)\b`},
	{"prompt_injection", `\b(ignore|disregard|forget|override)\s+(all\s+|any\s+|the\s+|your\s+)?(previous|prior|above|earlier|system)\s+(instructions?|prompts?|rules?)\b|\bsystem prompt\b|\bthis is (a |an )?(trivial|simple|easy|tiny)\b|\b(route|send|use)\b.{0,20}\b(cheap|cheapest|small|smallest|haiku|mini|nano)\s+(model|tier)\b`},
}

type riskSignal struct {
	category string
	re       *regexp.Regexp
}

var riskSignals = func() []riskSignal {
	out := make([]riskSignal, 0, len(RawRiskSignals))
	for _, r := range RawRiskSignals {
		out = append(out, riskSignal{category: r.Category, re: regexp.MustCompile(`(?i)` + r.Pattern)})
	}
	return out
}()

var camelBoundary = regexp.MustCompile(`([a-z0-9])([A-Z])`)

// splitIdentifiers turns code identifiers into words so risk vocabulary is
// found inside them: creditCardNumber → "credit Card Number", api_key →
// "api key".
func splitIdentifiers(prompt string) string {
	return strings.ReplaceAll(camelBoundary.ReplaceAllString(prompt, "$1 $2"), "_", " ")
}

// RiskCategories returns the sorted risk categories the prompt touches.
func RiskCategories(prompt string) []string {
	text := splitIdentifiers(prompt)
	var cats []string
	for _, s := range riskSignals {
		if s.re.MatchString(text) {
			cats = append(cats, s.category)
		}
	}
	sort.Strings(cats)
	return cats
}

// applyRiskFloor lifts a Trivial/Simple classification that touches a risk
// category to Medium (mid tier) and clears Confident, so no adapter acts on
// a confident downshift for it. Medium and Complex are left unchanged.
func applyRiskFloor(prompt string, c Classification) Classification {
	if c.Complexity >= Medium {
		return c
	}
	cats := RiskCategories(prompt)
	if len(cats) == 0 {
		return c
	}
	c.Complexity = Medium
	c.Confident = false
	c.RiskFloor = true
	c.Risk = cats
	return c
}
