// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package shellwrap

import (
	"strings"

	"github.com/tiagovilasboas/downshift-ai/internal/execwrap"
)

// ReasonQuotedRule is the skip code for a quoted command left unwrapped
// because a shell deny or ask rule holds a quote.
const ReasonQuotedRule = "quoted_rule"

// ShellRuleReader is implemented by adapters that can read the harness's own
// deny and ask rules for its shell tool. The adapter only reads them; the
// policy on them lives here.
type ShellRuleReader interface {
	// ShellDenyAskRules returns the shell tool's deny and ask rules as
	// written. A read error is returned, so the policy fails closed.
	ShellDenyAskRules() ([]string, error)
}

// refuseQuotedUnderQuotedRules keeps the quoted path off when any shell deny
// or ask rule holds ' or ". Such a rule can name one argv written many ways
// (`-m "wip"`, `-m 'wip'`, `-m w"i"p`), and the harness then sees only the
// rewritten string, so no comparison of strings can show it does not match:
// the command stays as typed. A rule that cannot be read fails closed too.
func refuseQuotedUnderQuotedRules(ev any, d execwrap.Decision) execwrap.Decision {
	if !d.Rewrite || d.Reason != execwrap.ReasonAllowlistedQuoted {
		return d
	}
	r, ok := ev.(ShellRuleReader)
	if !ok {
		return d
	}
	rules, err := r.ShellDenyAskRules()
	if err != nil || anyRuleQuoted(rules) {
		return execwrap.Decision{Family: d.Family, Reason: ReasonQuotedRule}
	}
	return d
}

func anyRuleQuoted(rules []string) bool {
	for _, r := range rules {
		if strings.ContainsAny(r, `'"`) {
			return true
		}
	}
	return false
}
