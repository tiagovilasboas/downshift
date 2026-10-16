// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"fmt"
	"strings"
)

// reasonWords is each reason code in plain words, for `downshift session --why`.
// Every code in reason_codes.go has an entry (a test enforces it).
var reasonWords = map[string]string{
	ReasonClassifierConfident:   "the classifier was confident about how big the task is",
	ReasonClassifierUnconfident: "the classifier was unsure, so moves that spend or save on doubt were held",
	ReasonNoSignal:              "nothing in the task pointed up or down, so the default size applied",
	ReasonSemanticBoost:         "meaning-based matching raised the complexity above the keyword read",
	ReasonUnsupportedLanguage:   "the language guard found text outside the English and Portuguese rule coverage, so confidence was withheld",
	ReasonRiskCategory:          "the task touches a risk area (secrets, auth, payments, personal data, destructive ops)",
	ReasonRiskFloor:             "a risk area set a minimum tier, so the task cannot go below it",
	ReasonGraphEscalation:       "the dependency graph showed a wide change, which raised the tier",
	ReasonNBUpshiftApplied:      "the second-opinion model raised the tier",
	ReasonNBDownshiftApplied:    "the second-opinion model lowered the tier for a trivial task",
	ReasonTierFloor:             "the project's tier floor raised the tier",
	ReasonCreditHeld:            "the credit gate refused the rewrite (quota exhausted, stale or unknown)",
	ReasonGuessHeld:             "the baseline was guessed (settings or the parent model), so only a small-tier downshift is written",
	ReasonPaused:                "routing was paused for this session: classified and logged, the spawn ran unchanged",
	ReasonScoreRouted:           "public capability scores picked the model",
	ReasonBaselineFromSettings:  "the session model was inferred from the configured model (env or settings), not seen in the transcript",
	ReasonHeldUnmapped:          "a guardrail this version cannot name held the decision",

	"HELD_UNCONFIDENT_DOWNSHIFT":        "a cheaper model was held: the classifier was not confident enough to downshift",
	"HELD_COMPLEX_DOWNSHIFT":            "a cheaper model was held: complex work is never moved to a lower tier",
	"HELD_FOREIGN_MODEL":                "nothing was written: the target model belongs to another harness",
	"HELD_RISK_DOWNSHIFT":               "a cheaper model was held: the task touches a risk area",
	"HELD_UNCONFIDENT_UNKNOWN_SMALL":    "a small model was held: the requested model was unknown and the read was unsure",
	"HELD_UNCONFIDENT_UNKNOWN_FRONTIER": "a frontier model was held: the requested model was unknown and the read was unsure",
	"HELD_UNCONFIDENT_UPSHIFT":          "a more expensive model was held: the read was too unsure to pay for it",
	"HELD_UNCONFIDENT_UNKNOWN_MID":      "a mid model was held: the requested model was unknown and the read was unsure",
}

// ReasonWords returns the plain-words form of a reason code, or the code
// itself when this version has no words for it.
func ReasonWords(code string) string {
	if w, ok := reasonWords[code]; ok {
		return w
	}
	return "no description in this version"
}

// isGuardrail marks the codes that stopped or undid a move, as opposed to the
// signals and lifts that shaped it.
func isGuardrail(code string) bool {
	return strings.HasPrefix(code, "HELD_") || code == ReasonCreditHeld || code == ReasonPaused
}

// Why explains the latest routing decision of the session in plain words: what
// was asked, what ran, the verdict and tier, every reason code and the
// guardrail that held it. It fits one screen. Events written before reason
// codes existed say so instead of guessing.
func Why(events []Event, sessionHash string) string {
	evs := sessionEvents(events, sessionHash)
	var last *Event
	n := 0
	facts := map[string]*childFacts{}
	for i := range evs {
		ev := evs[i]
		switch {
		case IsDecisionRow(ev):
			last, n = &evs[i], n+1
		case ev.LinkedDecision != "" && (ev.Outcome == OutcomeResolved || ev.Outcome == OutcomeUsage):
			f := facts[ev.LinkedDecision]
			if f == nil {
				f = &childFacts{}
				facts[ev.LinkedDecision] = f
			}
			if ev.Outcome == OutcomeResolved {
				f.resolved = &evs[i]
			} else {
				f.usage = &evs[i]
			}
		}
	}
	if last == nil {
		return "why · no subagent decision in this session yet\n"
	}
	f := facts[DecisionKey(*last)]
	if f == nil {
		f = &childFacts{}
	}
	var b strings.Builder
	fmt.Fprintf(&b, "why · subagent %d, the latest decision of the session\n", n)
	asked := "unknown"
	if last.FromModel != "" && last.FromModel != "unknown" {
		asked = ShortModel(last.FromModel)
	}
	row := func(k, v string) { fmt.Fprintf(&b, "  %-11s %s\n", k, v) }
	row("asked for", asked)
	row("ran on", rowModel(*last, f))
	verdict := last.Verdict
	if last.SafeVerdict != "" && last.SafeVerdict != last.Verdict {
		verdict += ", held to " + last.SafeVerdict
	}
	detail := strings.ToLower(last.Tier)
	if last.Complexity != "" {
		detail = strings.TrimPrefix(detail+", "+last.Complexity, ", ")
	}
	row("verdict", strings.TrimSpace(verdict+" ("+detail+")"))
	row("outcome", outcomeWords(*last))

	var shaped, guards []string
	for _, c := range last.ReasonCodes {
		if isGuardrail(c) {
			guards = append(guards, c)
		} else {
			shaped = append(shaped, c)
		}
	}
	if len(last.ReasonCodes) == 0 {
		b.WriteString("\n  This event predates reason codes, so only the fields above are known.\n")
		if len(last.Corrections) > 0 {
			fmt.Fprintf(&b, "  Guardrail rules recorded: %s\n", strings.Join(last.Corrections, ", "))
		}
		return b.String()
	}
	w := 0
	for _, c := range last.ReasonCodes {
		w = max(w, len(c))
	}
	section := func(title string, codes []string, none string) {
		fmt.Fprintf(&b, "\n%s\n", title)
		if len(codes) == 0 {
			fmt.Fprintf(&b, "  %s\n", none)
			return
		}
		for _, c := range codes {
			fmt.Fprintf(&b, "  %-*s  %s\n", w, c, ReasonWords(c))
		}
	}
	section("what shaped it", shaped, "no signal fired")
	section("what held it", guards, "no guardrail held this decision")
	return b.String()
}

func outcomeWords(ev Event) string {
	paused := hasCode(ev.ReasonCodes, ReasonPaused)
	switch {
	case ev.Outcome == OutcomeBaseline && paused:
		return "paused: spawn left unchanged"
	case ev.Outcome == OutcomeBaseline:
		return "control group: spawn left unchanged"
	case ev.Outcome == OutcomeBlocked:
		return "spawn blocked, the agent was asked to respawn on another model"
	case len(ev.Corrections) > 0 || hasHeld(ev.ReasonCodes):
		return "held: spawn left unchanged"
	case ev.Outcome == OutcomeAllow:
		return "spawn left unchanged"
	}
	if ev.FromModel != "" && ev.FromModel == ev.ToModel {
		return "spawn left unchanged"
	}
	return "model rewritten to " + ShortModel(ev.ToModel)
}

func hasHeld(codes []string) bool {
	for _, c := range codes {
		if strings.HasPrefix(c, "HELD_") {
			return true
		}
	}
	return false
}
