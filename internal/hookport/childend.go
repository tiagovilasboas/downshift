// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package hookport

import "github.com/tiagovilasboas/downshift/internal/telemetry"

// TokenUsage is the token count a child reported. Zero fields mean "not
// reported", never a measured zero.
type TokenUsage = telemetry.TokenUsage

// Where a ChildEnd was observed. The shared recorder uses it to decide which
// records the event may produce.
const (
	// SourcePayload: the hook payload itself reports the launch or end of a
	// spawn (resolved model, status). It may produce resolved, child_ended and
	// usage records.
	SourcePayload = "payload"
	// SourceTranscript: only a transcript read at the end of the child
	// reported the model and tokens. It produces a usage record and nothing
	// else: a stop event makes no claim about how the child ended.
	SourceTranscript = "transcript"
)

// ChildEnd is the harness-free fact "this child spawn ended (or was observed)".
// An adapter parses its own payload into it; the shared recorder
// (internal/childend) does everything else: pricing, linking to the decision,
// dedup and the child-end line. Fields not known stay at the zero value, and
// absence is never inferred.
type ChildEnd struct {
	Harness string
	// AgentKey is the harness id of the child, raw. The recorder hashes it.
	AgentKey string
	// ToolUseID is the id of the spawn tool call, raw. It links the end to
	// the exact decision made for that call.
	ToolUseID string
	// SessionID is the raw session id, when the payload carries one.
	SessionID string
	// Model is the model the usage ran on, as the payload or transcript reports
	// it; it wins over the decision's model when pricing.
	Model string
	// ResolvedModel is the model the harness says it chose for the child at
	// launch. It produces the "resolved" observation (and, when the decision
	// was a rewrite, the honored/refuted comparison). Empty when the event does
	// not report one.
	ResolvedModel string
	// Status is how the child ended; "" or ChildUnknown when the payload does
	// not say.
	Status telemetry.ChildStatus
	// Usage is nil when no token count was found.
	Usage *TokenUsage
	// Cumulative is true when Usage is the child's running total read from its
	// whole transcript. A child can stop, be sent another message and stop
	// again, so a later stop of an agent already priced records only the growth
	// since the last record. False for counts that cover one turn or call.
	Cumulative bool
	// Source is SourcePayload or SourceTranscript.
	Source string
	// IsChild is true when the event is a subagent spawn or stop, false when
	// another tool call merely carried usage. Only a child gets the end line.
	IsChild bool
	// Verification is "passed", "failed" or "none" when the harness transcript
	// let the adapter check the child's own verification; "" otherwise.
	Verification string
	// Effort is the reasoning effort the child ran at, as its transcript
	// reports it; "" when the harness does not say.
	Effort string
}

// ParseChildEndFunc reads one post-spawn or stop payload. ok is false when the
// payload is not a child end for this harness (another event, another tool,
// unparseable): the caller records nothing and does not fail.
type ParseChildEndFunc func(raw []byte) (ChildEnd, bool)
