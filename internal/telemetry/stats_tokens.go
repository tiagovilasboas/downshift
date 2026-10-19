// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"fmt"
	"io"
	"strings"
)

// tokenBreakdown is "1.2M (in 40k / out 90k / cache 1.1M)", or "0" for none.
func tokenBreakdown(c TokenClasses) string {
	if c.Total == 0 {
		return "0"
	}
	return fmt.Sprintf("%s (in %s / out %s / cache %s)", formatTokens(c.Total), formatTokens(c.Input), formatTokens(c.Output), formatTokens(c.Cache()))
}

// printTokensLine writes the one line next to the headline: the tokens that ran
// on a cheaper model, and on a pricier one. These are tokens that moved, not
// tokens saved: the dollar line above prices the same tokens on the requested
// model. With no observed harness it says so instead of printing zeros.
func printTokensLine(w io.Writer, t TokensRerouted) {
	if t.Totals == nil {
		fmt.Fprintf(w, "Tokens moved to a cheaper model: unobserved (no priced child usage in this window)\n")
		return
	}
	fmt.Fprintf(w, "Tokens moved to a cheaper model: %s · to a pricier model: %s\n",
		tokenBreakdown(t.Totals.Cheaper), tokenBreakdown(t.Totals.Pricier))
}

// printTokensDetail writes the per-harness and per-pair tables of --detail.
func printTokensDetail(w io.Writer, t TokensRerouted) {
	fmt.Fprintf(w, "\nTokens rerouted, by harness (%s)\n", t.Basis)
	for _, h := range t.HarnessOrder() {
		ht := t.ByHarness[h]
		switch ht.Status {
		case TokenStatusUnobserved:
			fmt.Fprintf(w, "  %-12s unobserved (usage source: %s)\n", h, ht.UsageSource)
			continue
		case TokenStatusNoneInRange:
			fmt.Fprintf(w, "  %-12s no priced usage records in this window (usage source: %s)\n", h, ht.UsageSource)
			continue
		}
		fmt.Fprintf(w, "  %s\n", h)
		for _, row := range []struct {
			name string
			c    *TokenClasses
		}{{"cheaper", ht.Cheaper}, {"pricier", ht.Pricier}, {"unchanged", ht.Unchanged}, {"unattributed", ht.Unattributed}} {
			fmt.Fprintf(w, "    %-12s %s  [%d records; cache read %s, write %s]\n",
				row.name, tokenBreakdown(*row.c), row.c.Records, formatTokens(row.c.CacheRead), formatTokens(row.c.CacheWrite))
		}
		for _, p := range ht.ByPair {
			fmt.Fprintf(w, "    %s -> %s  %s  %s\n", ShortModel(p.Requested), ShortModel(p.Ran), p.Direction, tokenBreakdown(p.TokenClasses))
		}
		mix := make([]string, 0, len(ht.ByModel))
		for _, m := range ht.ByModel {
			mix = append(mix, fmt.Sprintf("%s %s", ShortModel(m.Model), formatTokens(m.Total)))
		}
		fmt.Fprintf(w, "    token mix by model: %s\n", strings.Join(mix, " · "))
	}
	fmt.Fprintf(w, "  unchanged = ran on the requested model or at its price (allow, held, control, not honored); unattributed = no linked decision or no requested-model price.\n")
}
