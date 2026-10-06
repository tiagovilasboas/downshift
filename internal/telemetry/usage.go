// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"encoding/json"

	"github.com/tiagovilasboas/harness-downshift/internal/core"
)

// Real-cost usage extraction and pricing for PostToolUse hook payloads.
//
// Assumed Claude Code PostToolUse payload shape (UNCERTAINTY — no sample
// payload or schema was found in this repo as of Oct 2026; the only in-repo
// reference is the "PostToolUse hook planned" placeholder in PrintStats):
//
//	{
//	  "hook_event_name": "PostToolUse",
//	  "tool_name": "Task",
//	  "model": "<model id that actually ran>",
//	  "session_id": "<opaque session id>",
//	  "usage": {
//	    "input_tokens": 1234,
//	    "output_tokens": 567,
//	    "cache_creation_input_tokens": 100,
//	    "cache_read_input_tokens": 200
//	  }
//	}
//
// The four usage key names follow Anthropic's usage vocabulary. Because the
// shape is unverified, the parser is deliberately defensive:
//   - it accepts the usage object nested under "usage" AND flat top-level
//     keys (nested wins per key);
//   - only JSON numbers (and Go int kinds, for programmatic callers) are
//     accepted — strings, bools, nulls, and objects are ignored;
//   - negative values are clamped to zero (a provider never bills negative
//     tokens; a negative here means a corrupt or hostile payload);
//   - every other key is ignored, so future payload additions cannot break
//     parsing or leak prompt content into the event log.
//
// CachedTokens is the sum of cache_creation_input_tokens and
// cache_read_input_tokens: CostUSD bills cached tokens at the input rate,
// matching the TokenUsage contract in cost.go.

// Outcome values written by post-hoc cost linkage (v1: PostToolUse hook).
// The "baseline" value is reserved for a future symmetric hook that records
// what the unrouted baseline would have cost; Aggregate already excludes it.
//
// "resolved" records the model the harness actually chose for a spawn, read
// from the PostToolUse payload. It is an observation about an earlier decision
// (LinkedDecision), never a decision itself.
const (
	OutcomeUsage    = "usage"
	OutcomeBaseline = "baseline"
	OutcomeResolved = "resolved"
)

// IsCostOnlyOutcome reports whether an outcome value marks a post-hoc cost
// record rather than a routing decision. Cost records must never inflate
// decision counts in Aggregate.
func IsCostOnlyOutcome(outcome string) bool {
	return outcome == OutcomeUsage || outcome == OutcomeBaseline || outcome == OutcomeResolved
}

// DecisionKey identifies a decision event for post-hoc linkage: its
// correlation id, or timestamp+session for lines written without one.
func DecisionKey(ev Event) string {
	if ev.CorrelationID != "" {
		return ev.CorrelationID
	}
	return ev.Timestamp + "|" + ev.SessionID
}

// IsAppliedShift reports a decision event whose rewrite was written and
// changed the model. These are the only decisions a harness can honor or not.
func IsAppliedShift(ev Event) bool { return isShift(ev) }

// TokenUsageFromPayload extracts provider-reported token counts from a
// decoded PostToolUse stdin payload. It never fails: missing or malformed
// fields yield zero values, and unknown fields are ignored.
func TokenUsageFromPayload(m map[string]any) TokenUsage {
	if m == nil {
		return TokenUsage{}
	}
	nested, _ := m["usage"].(map[string]any)
	get := func(key string) int64 {
		if nested != nil {
			if v, ok := nested[key]; ok {
				if n := numberToInt64(v); n > 0 {
					return n
				}
				// Present-but-invalid nested value falls through to the
				// flat key rather than silently winning with zero.
				// (Zero and negative both mean "no usable value here".)
			}
		}
		return numberToInt64(m[key])
	}
	creation := get("cache_creation_input_tokens")
	read := get("cache_read_input_tokens")
	return TokenUsage{
		InputTokens:  get("input_tokens"),
		OutputTokens: get("output_tokens"),
		CachedTokens: creation + read,
	}
}

// numberToInt64 coerces a decoded JSON number to int64. Non-numbers
// (strings, bools, nil, maps, slices) and negatives return 0.
func numberToInt64(v any) int64 {
	switch n := v.(type) {
	case float64:
		if n <= 0 {
			return 0
		}
		return int64(n)
	case float32:
		if n <= 0 {
			return 0
		}
		return int64(n)
	case int:
		if n <= 0 {
			return 0
		}
		return int64(n)
	case int64:
		if n <= 0 {
			return 0
		}
		return n
	case int32:
		if n <= 0 {
			return 0
		}
		return int64(n)
	case uint64:
		return int64(n)
	case json.Number:
		if f, err := n.Float64(); err == nil && f > 0 {
			return int64(f)
		}
		return 0
	default:
		return 0
	}
}

// FillRealCost prices the token usage already stored on ev and fills both
// cost pointers via CostUSD. Usage is read with TokenUsageOrZero, so missing
// or negative token fields contribute zero, never garbage.
//
// Callers should only invoke this when the spend is real: total tokens > 0
// and the routed model resolved against the catalog. A zero-usage event
// would otherwise register as a $0 "real cost" and inflate RealCostEvents
// with a meaningless record.
func FillRealCost(ev *Event, baseline, routed core.Model) {
	if ev == nil {
		return
	}
	usage := ev.TokenUsageOrZero()
	actual := CostUSD(routed, usage)
	base := CostUSD(baseline, usage)
	ev.ActualCostUSD = &actual
	ev.BaselineCostUSD = &base
}
