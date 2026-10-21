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

// stripTryOptions removes --harness X and --model X (or --harness=X) from try
// arguments and returns them with the remaining positional arguments. The
// positional form "<prompt> [harness] [current-model]" keeps working; an
// option wins over the positional slot it names. Any other leading "--flag"
// is reported so it is never read as the task prompt.
func stripTryOptions(args []string) (rest []string, harness, model, badFlag string) {
	for i := 0; i < len(args); i++ {
		a := args[i]
		name, val, hasVal := strings.Cut(a, "=")
		switch name {
		case "--harness", "--model":
			if !hasVal {
				if i+1 >= len(args) {
					return nil, "", "", a
				}
				i++
				val = args[i]
			}
			if name == "--harness" {
				harness = val
			} else {
				model = val
			}
		default:
			if len(rest) == 0 && strings.HasPrefix(a, "--") {
				return nil, "", "", a
			}
			rest = append(rest, a)
		}
	}
	return rest, harness, model, ""
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
	printTry(os.Stdout, res, known, d, prompt, harness, current, assumed, detail)
	return 0
}

func printTry(w io.Writer, res tryResult, known bool, d core.Decision, prompt, harness, current string, assumed, detail bool) {
	if assumed {
		fmt.Fprintf(w, "assumed current: %s (harness default)\n", current)
	}
	note := tryNotApplied(res, known, d, harness)
	if note != "" {
		// The hook would leave the spawn alone: do not claim a move it will not make.
		fmt.Fprintf(w, "%s\n", tryHeadlineNotApplied(d))
	} else {
		fmt.Fprintf(w, "%s\n", tryHeadline(d))
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
	if note != "" {
		// The hook would not apply the move: the tokens stay where they were asked.
		fmt.Fprintf(w, "→ %s task → %s (not applied, tokens stay on %s)\n", d.Complexity, d.Model.ID, d.Baseline.CurrentModel.ID)
		return
	}
	fmt.Fprintf(w, "→ %s\n", trySummary(d))
}

// trySummary is the last --detail line. A downshift names where its tokens
// would run instead of the list-price ratio Decision.Summary quotes: one
// decision has no tokens yet, and dollars are priced only by downshift report.
func trySummary(d core.Decision) string {
	if d.Verdict == core.VerdictDownshift {
		return fmt.Sprintf("%s task → downshift to %s (%s)", d.Complexity, d.Model.ID, tokensMove(d))
	}
	return d.Summary()
}

// tokensMove says where the child's tokens would run against the model the
// harness asked for, with no price and no percentage (owner decision
// 2026-10-19: per-decision views show tokens, not dollars). "Cheaper model"
// needs a catalog price on both sides; without one only the tier is known.
func tokensMove(d core.Decision) string {
	switch d.Verdict {
	case core.VerdictDownshift:
		if d.Savings > 0 {
			return "its tokens would run on a cheaper model than requested"
		}
		return "its tokens would run on a lower tier than requested"
	case core.VerdictUpshift:
		return "its tokens would run on a pricier tier than requested"
	}
	return ""
}

// tryHeadline is the first answer line: the tier, then either the move from
// the current model and where its tokens would run, or that it stays.
func tryHeadline(d core.Decision) string {
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
		return fmt.Sprintf("→ %s: %s → %s · %s", d.Tier, cur, rec, tokensMove(d))
	case core.VerdictUpshift:
		return fmt.Sprintf("→ %s: %s → %s · %s (needs more torque)", d.Tier, cur, rec, tokensMove(d))
	}
	return fmt.Sprintf("→ %s: stays on %s", d.Tier, cur)
}

// tryHeadlineNotApplied is the headline for a move the hook would not apply:
// the tier and the model the decision points at, and that the tokens stay on
// the current model.
func tryHeadlineNotApplied(d core.Decision) string {
	rec := telemetry.ShortModel(d.Model.ID)
	cur := telemetry.ShortModel(d.Baseline.CurrentModel.ID)
	if d.Baseline.CurrentModel.ID == "" {
		return fmt.Sprintf("→ %s: %s (not applied)", d.Tier, rec)
	}
	return fmt.Sprintf("→ %s: %s → %s (not applied, tokens stay on %s)", d.Tier, cur, rec, cur)
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
