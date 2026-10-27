// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package fromcore

import (
	"sort"

	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// heldReason maps each self-check rule ID to its hold code.
var heldReason = map[string]string{
	core.RuleUnconfidentDowngrade:       telemetry.ReasonHeldUnconfidentDownshift,
	core.RuleComplexDowngrade:           telemetry.ReasonHeldComplexDownshift,
	core.RuleForeignModel:               telemetry.ReasonHeldForeignModel,
	core.RuleRiskDowngrade:              telemetry.ReasonHeldRiskDownshift,
	core.RuleUnconfidentUnknownSmall:    telemetry.ReasonHeldUnconfidentUnknownSmall,
	core.RuleUnconfidentUnknownFrontier: telemetry.ReasonHeldUnconfidentUnknownFrontier,
	core.RuleUnconfidentUpshift:         telemetry.ReasonHeldUnconfidentUpshift,
	core.RuleUnconfidentUnknownMid:      telemetry.ReasonHeldUnconfidentUnknownMid,
}

// ReasonCodes returns the sorted, de-duplicated reason codes for d: the
// signals that fired, the deterministic lifts that applied and the guardrails
// that held the decision. Shadow-only opinions (NB not applied, adapt memory)
// are not reasons; they have their own event fields.
func ReasonCodes(d core.Decision) []string {
	set := make(map[string]bool)
	if d.Confident {
		set[telemetry.ReasonClassifierConfident] = true
	} else {
		set[telemetry.ReasonClassifierUnconfident] = true
	}
	if d.NoSignal {
		set[telemetry.ReasonNoSignal] = true
	}
	if d.TaskNotVisible {
		set[telemetry.ReasonTaskNotVisible] = true
	}
	if d.UnsupportedLanguage {
		set[telemetry.ReasonUnsupportedLanguage] = true
	}
	if d.Guards.SemanticLift {
		set[telemetry.ReasonSemanticBoost] = true
	}
	if d.Guards.TaskFocus {
		set[telemetry.ReasonTaskFocus] = true
	}
	if len(d.Risk) > 0 {
		set[telemetry.ReasonRiskCategory] = true
	}
	if d.Guards.RiskFloor {
		set[telemetry.ReasonRiskFloor] = true
	}
	if d.Shadow.NBUpshift.Applied {
		set[telemetry.ReasonNBUpshiftApplied] = true
	}
	if d.Shadow.NBDownshift.Applied {
		set[telemetry.ReasonNBDownshiftApplied] = true
	}
	if d.Guards.PolicyFloor {
		set[telemetry.ReasonTierFloor] = true
	}
	if d.Session.CreditHeld {
		set[telemetry.ReasonCreditHeld] = true
	}
	if d.Baseline.GuessHeld {
		set[telemetry.ReasonGuessHeld] = true
	}
	if d.Shadow.ScoreApplied {
		set[telemetry.ReasonScoreRouted] = true
	}
	if d.Baseline.FromSettings {
		set[telemetry.ReasonBaselineFromSettings] = true
	}
	if d.Guards.AgentTypeHint {
		set[telemetry.ReasonAgentTypeHint] = true
	}
	if d.Baseline.Pinned {
		set[telemetry.ReasonAgentPinsModel] = true
	}
	for _, rule := range d.Corrections {
		if code, ok := heldReason[rule]; ok {
			set[code] = true
		} else {
			set[telemetry.ReasonHeldUnmapped] = true
		}
	}
	codes := make([]string, 0, len(set))
	for code := range set {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	return codes
}
