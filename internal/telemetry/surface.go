// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

// Spawn-surface values. Each one is absolute only for the decision whose own
// hook observed it. A nil Honor or Usage func never produces observed.
const (
	HonorObserved   = "observed"
	HonorUnobserved = "unobserved"
	UsageObserved   = "observed"
	UsageUnobserved = "unobserved"
	QuotaAvailable  = "available"
	QuotaHeld       = "held"
	QuotaUnknown    = "unknown"
)

// SpawnSurface is the harness-agnostic result of one routing decision.
// Honor and Usage are projected at read time from linked records. Quota is
// projected from the decision's quota status and whether the credit gate
// held the rewrite.
type SpawnSurface struct {
	Honor string `json:"honor"`
	Quota string `json:"quota"`
	Usage string `json:"usage"`
}

// ProjectQuota maps internal quota evidence onto the spawn surface.
// A credit-gate refusal is held even when the retained model's status string
// is available. exhausted and stale are holds on their own. available is the
// only credited state, and only when the gate did not refuse the rewrite.
// unknown with no credit observation stays unknown.
func ProjectQuota(status string, creditHeld bool) string {
	if creditHeld {
		return QuotaHeld
	}
	switch status {
	case QuotaAvailable:
		return QuotaAvailable
	case "exhausted", "stale":
		return QuotaHeld
	default:
		return QuotaUnknown
	}
}

// SurfaceFor projects one decision. honorObserved is true only when a linked
// resolved record has rewrite_honored true. usageObserved is true only when
// a linked usage record carries that child's token counts. Neither flag is
// inferred from a later spawn or from catalog savings.
func SurfaceFor(ev Event, honorObserved, usageObserved bool) SpawnSurface {
	honor := HonorUnobserved
	if honorObserved {
		honor = HonorObserved
	}
	usage := UsageUnobserved
	if usageObserved {
		usage = UsageObserved
	}
	return SpawnSurface{
		Honor: honor,
		Quota: ProjectQuota(ev.QuotaStatus, ev.CreditHeld),
		Usage: usage,
	}
}

// SurfaceKey joins a decision to later honor and usage records from the same harness.
func SurfaceKey(ev Event) string {
	return ev.Harness + "\n" + DecisionKey(ev)
}

// ObservedLinks returns decision keys whose own port recorded honor or child
// tokens. A resolved row counts as honor only when rewrite_honored is true.
// A usage row counts only when it carries token counts. Catalog cost without
// tokens is not usage. The maps are keyed by SurfaceKey.
func ObservedLinks(events []Event) (honor, usage map[string]bool) {
	honor = map[string]bool{}
	usage = map[string]bool{}
	for _, ev := range events {
		if ev.Harness == "" || ev.LinkedDecision == "" {
			continue
		}
		key := ev.Harness + "\n" + ev.LinkedDecision
		if ev.Outcome == OutcomeResolved && ev.RewriteHonored != nil && *ev.RewriteHonored {
			honor[key] = true
		}
		if ev.Outcome == OutcomeUsage && hasChildTokens(ev) {
			usage[key] = true
		}
	}
	return honor, usage
}

func hasChildTokens(ev Event) bool {
	return ev.InputTokens != nil || ev.OutputTokens != nil || ev.CachedTokens != nil
}

// ObservedChildTokens sums input, output, and cached tokens only from usage
// events that already count as child tokens (hasChildTokens). records is the
// count of those usage events. A nil pointer adds nothing. records == 0 means
// unobserved, not a measured zero. Catalog USD is not tokens.
func ObservedChildTokens(events []Event) (sum int64, records int) {
	for _, ev := range events {
		if ev.Outcome != OutcomeUsage || !hasChildTokens(ev) {
			continue
		}
		records++
		if ev.InputTokens != nil {
			sum += *ev.InputTokens
		}
		if ev.OutputTokens != nil {
			sum += *ev.OutputTokens
		}
		if ev.CachedTokens != nil {
			sum += *ev.CachedTokens
		}
	}
	return sum, records
}

// SavingsCredit reports whether an applied downshift may add estimated savings.
// unknown and held are not credited routes.
func SavingsCredit(ev Event) bool {
	return ProjectQuota(ev.QuotaStatus, ev.CreditHeld) == QuotaAvailable
}
