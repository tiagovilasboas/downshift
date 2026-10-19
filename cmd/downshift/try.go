// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// stripDetail removes the --detail flag from try arguments, wherever it sits.
func stripDetail(args []string) ([]string, bool) {
	out := make([]string, 0, len(args))
	detail := false
	for _, a := range args {
		if a == "--detail" {
			detail = true
			continue
		}
		out = append(out, a)
	}
	return out, detail
}

// harnessDefaultModel is the model a session on harness runs when nothing else
// is said: the catalog frontier model of that harness. Empty when unknown.
func harnessDefaultModel(resolver core.Resolver, harness string) string {
	return resolver.ModelFor(harness, core.TierFrontier).ID
}

// runTryReport asks the harness hook what it would do for prompt and prints
// it. Without a current model it compares against the harness default and
// says so. The first lines are the answer; the classifier fields are under
// --detail.
func runTryReport(resolver core.Resolver, harness, prompt, current string, detail bool) int {
	assumed := false
	if current == "" {
		if def := harnessDefaultModel(resolver, harness); def != "" {
			current, assumed = def, true
		}
	}
	res, known := tryHook(harness, prompt, current, resolver)
	d := res.decision
	if !known {
		d = core.Route(prompt, harness, current, resolver)
	}
	printTry(os.Stdout, res, known, d, prompt, harness, current, assumed, detail, pricesAsOf(resolver))
	return 0
}

func printTry(w io.Writer, res tryResult, known bool, d core.Decision, prompt, harness, current string, assumed, detail bool, asOf string) {
	if assumed {
		fmt.Fprintf(w, "assumed current: %s (harness default)\n", current)
	}
	note := tryNotApplied(res, known, d, harness)
	if note != "" {
		// The hook would leave the spawn alone: do not quote a saving it will not make.
		fmt.Fprintf(w, "%s\n", tryHeadlineNotApplied(d))
	} else {
		fmt.Fprintf(w, "%s\n", tryHeadline(d, asOf))
	}
	fmt.Fprintf(w, "why: %s task, %s\n", strings.ToLower(d.Complexity.String()), telemetry.ReasonBrief(telemetry.ReasonCodes(d)))
	if note != "" {
		fmt.Fprintf(w, "note: %s\n", note)
	}
	if !detail {
		fmt.Fprintf(w, "(details: add --detail)\n")
		return
	}
	fmt.Fprintf(w, "\n")
	fmt.Fprintf(w, "Task:       %s\n", prompt)
	fmt.Fprintf(w, "Complexity: %s\n", d.Complexity)
	fmt.Fprintf(w, "Intent:     %s\n", d.Intent)
	fmt.Fprintf(w, "Needs tier: %s\n", d.Tier)
	fmt.Fprintf(w, "Recommend:  %s\n", d.Model.ID)
	if d.Baseline.CurrentModel.ID != "" {
		fmt.Fprintf(w, "Current:    %s\n", d.Baseline.CurrentModel.ID)
	}
	fmt.Fprintf(w, "Verdict:    %s\n", d.Verdict)
	fmt.Fprintf(w, "Confident:  %t\n", d.Confident)
	if len(d.Corrections) > 0 {
		fmt.Fprintf(w, "Held by:    %s\n", strings.Join(d.Corrections, ", "))
	}
	switch {
	case !known:
		fmt.Fprintf(w, "Rewrite:    unknown harness %q (no hook adapter)\n", harness)
	case res.blocked:
		fmt.Fprintf(w, "Hook:       block (exit 2) → respawn with %s\n", res.written)
	case res.written != "":
		fmt.Fprintf(w, "Rewrite:    yes → %s\n", res.written)
	case d.Session.Unknown:
		fmt.Fprintf(w, "Rewrite:    no: session unknown. Add \"%s\": [<model ids>] to %s (see docs/reference/session-models.md)\n",
			harness, sessionModelsPathForDisplay())
	case d.Intent == core.PreservedIntent:
		fmt.Fprintf(w, "Rewrite:    no (current model is explicit_only)\n")
	case len(d.Corrections) > 0:
		fmt.Fprintf(w, "Rewrite:    no (held by guardrail)\n")
	default:
		fmt.Fprintf(w, "Rewrite:    no (keep current model)\n")
	}
	if res.effort != "" {
		fmt.Fprintf(w, "Effort:     → %s\n", res.effort)
	}
	fmt.Fprintf(w, "→ %s\n", d.Summary())
}

// tryHeadline is the first answer line: the tier, then either the move from
// the current model or that it stays.
func tryHeadline(d core.Decision, asOf string) string {
	rec := telemetry.ShortModel(d.Model.ID)
	cur := telemetry.ShortModel(d.Baseline.CurrentModel.ID)
	if d.Baseline.CurrentModel.ID == "" {
		return fmt.Sprintf("→ %s: %s", d.Tier, rec)
	}
	if d.Model.ID == d.Baseline.CurrentModel.ID || d.Verdict == core.VerdictOK {
		return fmt.Sprintf("→ %s: stays on %s", d.Tier, cur)
	}
	switch d.Verdict {
	case core.VerdictDownshift:
		if d.Savings <= 0 {
			// No catalog price for one side (e.g. tier aliases): do not claim 0%.
			return fmt.Sprintf("→ %s: %s → %s (cheaper tier)", d.Tier, cur, rec)
		}
		return fmt.Sprintf("→ %s: %s → %s (~%.0f%% cheaper)%s", d.Tier, cur, rec, d.Savings*100, estimateSuffix(asOf))
	case core.VerdictUpshift:
		return fmt.Sprintf("→ %s: %s → %s (needs more torque)", d.Tier, cur, rec)
	}
	return fmt.Sprintf("→ %s: stays on %s", d.Tier, cur)
}

// tryHeadlineNotApplied is the headline for a move the hook would not apply:
// the tier and the model the decision points at, with no saving claimed.
func tryHeadlineNotApplied(d core.Decision) string {
	rec := telemetry.ShortModel(d.Model.ID)
	cur := telemetry.ShortModel(d.Baseline.CurrentModel.ID)
	if d.Baseline.CurrentModel.ID == "" {
		return fmt.Sprintf("→ %s: %s (not applied)", d.Tier, rec)
	}
	return fmt.Sprintf("→ %s: %s → %s (not applied, no saving)", d.Tier, cur, rec)
}

// pricesAsOf is the date of the resolver's embedded list prices, or "" when
// the resolver does not carry one.
func pricesAsOf(resolver core.Resolver) string {
	if p, ok := resolver.(interface{ PricesAsOf() string }); ok {
		return p.PricesAsOf()
	}
	return ""
}

// estimateSuffix labels a saving as an estimate at list prices and names
// their date. Empty when there is no date.
func estimateSuffix(asOf string) string {
	if asOf == "" {
		return ""
	}
	return " (est., list prices as of " + asOf + ")"
}

// tryNotApplied says why the hook would leave the spawn alone although the
// decision is a move. Empty when the move would apply or there is no move.
func tryNotApplied(res tryResult, known bool, d core.Decision, harness string) string {
	if !known || res.blocked || res.written != "" {
		return ""
	}
	if d.Verdict != core.VerdictDownshift && d.Verdict != core.VerdictUpshift {
		return ""
	}
	switch {
	case d.Session.Unknown:
		return fmt.Sprintf("not rewritten: session models unknown for %s (see docs/reference/session-models.md)", harness)
	case d.Intent == core.PreservedIntent:
		return "not rewritten: the current model is explicit_only"
	case d.Session.CreditHeld:
		return "not rewritten: the credit gate refused it (quota exhausted, stale or unknown)"
	case len(d.Corrections) > 0:
		return "not rewritten: held by a guardrail"
	}
	return "not rewritten: the session's model list does not allow it (see docs/reference/session-models.md)"
}
