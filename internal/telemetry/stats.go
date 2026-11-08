// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

// Stats is the aggregated view of all recorded events.
type Stats struct {
	Total       int
	Downshifted int
	Upshifted   int
	OK          int
	Unknown     int

	// UsageRecords counts all post-hoc usage records, including unmatched ones.
	// UsageLinked counts distinct usage records whose references resolve to
	// decisions in the same harness and compatible session. Records are deduped
	// by agent hash when supplied, otherwise their own event identity.
	// Baseline counts unrouted events (outcome "baseline"): a session
	// pause, or a line an older binary wrote. They were classified but never
	// applied, excluded from decision counts so before/after comparison
	// stays honest.
	UsageRecords int
	UsageLinked  int
	Baseline     int

	// PolicyFloor counts decisions an operator tier floor raised.
	PolicyFloor int
	// ByEntry counts decisions by spawn-time entry (hook, mod). Empty on
	// events written before the field existed are counted under EntryHook.
	ByEntry map[string]int
	// DuplicateToolUse counts distinct tool_use_hash values that appear on
	// more than one decision (both entry points live on the same spawn).
	DuplicateToolUse int
	// GatewayObserved counts decisions an allowlisted OpenRouter row would
	// have served; GatewaySlugs counts them per slug. Shadow only.
	GatewayObserved int
	GatewaySlugs    map[string]int
	// ChildStatus counts how children ended (child_ended events), by status.
	ChildStatus map[ChildStatus]int
	// RespawnAfterFailure counts failed children followed by another spawn in
	// the same session within respawnWindow. A hint, not a verdict.
	RespawnAfterFailure int

	// ReasonCodes counts decisions per reason code. Events written before
	// the decision trace carry no codes and add nothing here.
	ReasonCodes map[string]int
	// FrontierDecisions counts decisions that picked the frontier tier.
	// FrontierOnDoubt counts those that rest on an unconfident classification
	// with no deterministic lift (risk floor, graph escalation, NB upshift, tier
	// floor): the silent-overspend shape. It should stay at zero.
	FrontierDecisions int
	FrontierOnDoubt   int

	// Corrected counts events where a guardrail held the decision: the
	// classified verdict stands in the log, but adapters applied the safe
	// action (see Event.Corrections for the rule IDs).
	Corrected int

	// NotApplied counts DOWNSHIFT/UPSHIFT decisions the hook did not apply
	// (outcome allow, blocked, or held by a guardrail). They are excluded
	// from Downshifted/Upshifted.
	NotApplied int

	// ByComplexity counts decisions per complexity class.
	ByComplexity map[string]int
	// ByAgentType counts decisions per subagent type, and how many of them had no
	// known baseline model. Events without a type are not counted here.
	ByAgentType map[string]AgentTypeCount
	// ModelMix counts decisions per model the child ended up on (final_model,
	// short family name). Decisions with no recorded model add nothing.
	ModelMix map[string]int

	// RewriteShifted counts rewrite_emitted events that asked for a different model.
	// RewriteHonored is the inferred count from CountInferredHonored. The
	// report does not print it as honor. HonorObserved counts applied rewrites with
	// a linked resolved record whose rewrite_honored is true; HonorUnobserved the
	// applied rewrites with none. Spawns left unchanged are in neither.
	RewriteShifted  int
	RewriteHonored  int
	HonorObserved   int
	HonorUnobserved int
	// HonorRefuted counts applied rewrites a linked resolved record refutes:
	// the harness reported another model. They are observed, not unobserved.
	HonorRefuted    int
	UsageObserved   int
	UsageUnobserved int
	QuotaAvailable  int
	QuotaHeld       int
	QuotaUnknown    int

	// ObservedChildTokens is the sum of input, output, and cached tokens from
	// usage events that carry token counts. ObservedChildUsage is how many
	// such usage events contributed. Zero usage means unobserved, not measured zero.
	ObservedChildTokens int64
	ObservedChildUsage  int

	// Routing is the token view of routing, per harness: the billable tokens
	// that ran below the session model (see routing_tokens.go). Counts only;
	// never summed across harnesses.
	Routing RoutingView
}
