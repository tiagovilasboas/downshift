// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package opencode — post-spawn observations (wire v1).
//
// An observation call receives NDJSON on stdin: one full envelope per line,
// in arrival order, for one child stream:
//
//	{"v":1,"kind":"observation","correlation":{"child":"<hex>"},
//	 "event":"switch_attempt|model_selected|child_context|child_done",
//	 "from":"…","to":"…","model":"…","previous":"…","result":"ok",
//	 "api":"…","status":"…","usage":{…},"seq":1,"tsUtc":"…"}
//
// Allow-listed fields only; unknown fields are dropped by the strict decode,
// bad lines are dropped silently, and model fields are id strings (the plugin
// projects the harness events before sending them). Arrival order is the
// ordering contract; seq and tsUtc are not trusted for it.
//
// Honor reports the child's resolved model from the observed switch chain:
// switch_attempt(result:"ok") → model_selected{model:X} → first
// child_context{X}. The resolved model is the first child_context model; the
// shared recorder compares it against the decision's target and refutes a
// mismatch. No switch chain, no claim: absence is never turned into a
// fabricated resolved model.
//
// Usage is PROVEN for the child: the plugin sums OpenCode's per-step token
// report (session.step.ended, one count per assistant message) for the child
// session only, and sends it on child_done. A child that never reports usage
// stays unobserved, never a measured zero. Whether OpenCode's output already
// includes reasoning tokens is NOT PROVEN, so reasoning is parsed and not
// summed into any figure.
package opencode

import (
	"bytes"
	"fmt"

	"github.com/tiagovilasboas/downshift-ai/internal/childend"
	"github.com/tiagovilasboas/downshift-ai/internal/core"
	"github.com/tiagovilasboas/downshift-ai/internal/hookport"
	"github.com/tiagovilasboas/downshift-ai/internal/telemetry"
)

// Honor is the port's post-spawn resolution reader. It scans one observation
// batch, and when arrival order proves the switch chain it hands the shared
// recorder a payload-sourced ChildEnd carrying the resolved model, which
// links the observation to the decision made for this child and refutes a
// mismatch. Without the chain nothing is recorded: the batch is silent, not
// evidence of anything.
func Honor(raw []byte, binaryVersion string, r core.Resolver) (any, string) {
	var c claim
	c.switchPos, c.selectedPos, c.contextPos = -1, -1, -1
	c.scan(raw)
	note := ""
	if c.dropped > 0 {
		note = fmt.Sprintf("%d invalid observation line(s) dropped", c.dropped)
	}
	// One batch carries one child; a batch without a valid child id cannot
	// link anything.
	if c.child == "" {
		return nil, note
	}
	resolved, ok := c.observed()
	if !ok {
		return nil, note
	}
	childend.Record(hookport.ChildEnd{
		Harness: harnessID,
		// The spawn correlation is the one id both sides share: the decision
		// recorded it as its session hash and tool-use hash, so this record
		// meets that decision and no sibling's.
		AgentKey:      c.child,
		ToolUseID:     c.child,
		SessionID:     c.child,
		ResolvedModel: resolved,
		Source:        hookport.SourcePayload,
		IsChild:       true,
	}, binaryVersion, r)
	return nil, note
}

// ParseChildEnd is the `downshift end opencode` parser: it reads one
// observation batch and takes the first child_done envelope in it. The model
// and status the envelope reports are the end facts; ResolvedModel is
// deliberately not set here, because a resolution claim needs the switch
// chain Honor observes, and the payload does not report one.
func ParseChildEnd(raw []byte) (hookport.ChildEnd, bool) {
	lines := bytes.Split(raw, []byte("\n"))
	if len(lines) > maxObservationLines {
		lines = lines[:maxObservationLines]
	}
	for _, line := range lines {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		obs, ok := parseObservation(line)
		if !ok || obs.Event != "child_done" {
			continue
		}
		child := obs.Correlation.Child
		ce := hookport.ChildEnd{
			Harness:   harnessID,
			AgentKey:  child,
			ToolUseID: child,
			SessionID: child,
			Model:     obs.Model,
			Status:    telemetry.ChildStatusFromText(obs.Status, telemetry.ChildStopped),
			Source:    hookport.SourcePayload,
			IsChild:   true,
		}
		if obs.Usage != nil {
			var cached, cacheWrite int64
			if obs.Usage.Cache != nil {
				cached = obs.Usage.Cache.Read + obs.Usage.Cache.Write
				cacheWrite = obs.Usage.Cache.Write
			}
			ce.Usage = &hookport.TokenUsage{
				InputTokens:      obs.Usage.Input,
				OutputTokens:     obs.Usage.Output,
				CachedTokens:     cached,
				CacheWriteTokens: cacheWrite,
			}
		}
		return ce, true
	}
	return hookport.ChildEnd{}, false
}

// Port registers the observation seam: Honor reads the switch chain and
// ParseChildEnd feeds `downshift end opencode`, which carries the child's
// token usage when the child_done envelope has one (see the package comment).
// EndLine stays nil: this harness has no verified way to show the child-end
// line.
func Port() hookport.Port {
	return hookport.Port{ID: harnessID, Honor: Honor, ParseChildEnd: ParseChildEnd}
}
