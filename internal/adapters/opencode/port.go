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
// Usage is NOT PROVEN and stays unobserved. The observed surfaces were the
// session bus events (session.usage.updated, session.step.ended) and the
// parent tool.success text; tool.execute.after was absent from every F0 run
// and probe, and no token seam has been verified end to end. Until one is,
// the port registers no usage func and opencode records the child end and
// status but never a cost: unverified tokens could belong to the parent
// thread. Do not treat this as a measured zero — it is unobserved.
package opencode

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/tiagovilasboas/downshift/internal/childend"
	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/hookport"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// Observation limits: one line cap, one batch line cap, and the same field
// caps as the route side for ids and labels.
const (
	maxObservationLineBytes = 8 << 10
	maxObservationLines     = 512
	maxObsModelLen          = 128
	maxObsAPILen            = 64
	maxObsLabelLen          = 32
	// maxUsageValue caps one token count; anything above it means the line
	// is not a plausible token report and the usage is dropped.
	maxUsageValue = int64(1) << 40
)

// Observation event names of wire v1.
var observationEvents = map[string]bool{
	"switch_attempt": true,
	"model_selected": true,
	"child_context":  true,
	"child_done":     true,
}

// observation is one allow-listed observation envelope. Unknown fields are
// dropped by the decode; model fields are id strings.
type observation struct {
	V           int             `json:"v"`
	Kind        string          `json:"kind"`
	Correlation RequestRelation `json:"correlation"`
	Event       string          `json:"event"`
	From        string          `json:"from"`
	To          string          `json:"to"`
	Model       string          `json:"model"`
	Previous    string          `json:"previous"`
	Result      string          `json:"result"`
	API         string          `json:"api"`
	Status      string          `json:"status"`
	Usage       *obsUsage       `json:"usage"`
	Seq         int             `json:"seq"`
	TsUtc       string          `json:"tsUtc"`
}

// obsUsage is the child_done token report. Whether reasoning tokens are
// already included in output is NOT PROVEN, so they are parsed and
// deliberately not summed anywhere.
type obsUsage struct {
	Input     int64     `json:"input"`
	Output    int64     `json:"output"`
	Reasoning int64     `json:"reasoning"`
	Cache     *obsCache `json:"cache"`
}

// obsCache splits the cached input into read and write parts.
type obsCache struct {
	Read  int64 `json:"read"`
	Write int64 `json:"write"`
}

// parseObservation decodes one line strictly. ok is false for anything that
// is not a bounded, well-formed v1 observation envelope.
func parseObservation(line []byte) (observation, bool) {
	if len(line) == 0 || len(line) > maxObservationLineBytes {
		return observation{}, false
	}
	var obs observation
	if err := json.Unmarshal(line, &obs); err != nil {
		return observation{}, false
	}
	if obs.V != 1 || obs.Kind != "observation" || !observationEvents[obs.Event] {
		return observation{}, false
	}
	if !childIDPattern.MatchString(obs.Correlation.Child) || len(obs.Correlation.Parent) > maxCorrelationLen {
		return observation{}, false
	}
	if len(obs.From) > maxObsModelLen || len(obs.To) > maxObsModelLen ||
		len(obs.Model) > maxObsModelLen || len(obs.Previous) > maxObsModelLen {
		return observation{}, false
	}
	if len(obs.API) > maxObsAPILen || len(obs.Result) > maxObsLabelLen ||
		len(obs.Status) > maxObsLabelLen || len(obs.TsUtc) > maxTsUtcLen {
		return observation{}, false
	}
	if !validUsage(obs.Usage) {
		return observation{}, false
	}
	return obs, true
}

// validUsage bounds every token count; a negative or absurd number makes the
// line invalid rather than recording a mangled count.
func validUsage(u *obsUsage) bool {
	if u == nil {
		return true
	}
	if u.Input < 0 || u.Output < 0 || u.Reasoning < 0 ||
		u.Input > maxUsageValue || u.Output > maxUsageValue || u.Reasoning > maxUsageValue {
		return false
	}
	if u.Cache != nil && (u.Cache.Read < 0 || u.Cache.Write < 0 ||
		u.Cache.Read > maxUsageValue || u.Cache.Write > maxUsageValue) {
		return false
	}
	return true
}

// claim tracks the ordered switch chain as lines arrive.
type claim struct {
	child string
	// switchPos, selectedPos and contextPos are the line positions of the
	// first ok switch, the first non-empty model_selected and the first
	// child_context with a model; -1 means not seen.
	switchPos   int
	selectedPos int
	contextPos  int
	contextID   string
	// dropped counts lines that are not well-formed observations of this
	// batch's child.
	dropped int
}

// observed reports whether the arrival order proves
// switch_attempt(result:"ok") < model_selected(model) < child_context(model)
// and returns the first child_context model, the child's resolved model.
func (c *claim) observed() (string, bool) {
	if c.switchPos < 0 || c.selectedPos < 0 || c.contextPos < 0 {
		return "", false
	}
	if c.switchPos < c.selectedPos && c.selectedPos < c.contextPos {
		return c.contextID, true
	}
	return "", false
}

// scan walks the batch once, in arrival order, keeping only this child's
// lines.
func (c *claim) scan(raw []byte) {
	lines := bytes.Split(raw, []byte("\n"))
	if len(lines) > maxObservationLines {
		lines = lines[:maxObservationLines]
	}
	for i, line := range lines {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		obs, ok := parseObservation(line)
		if !ok {
			c.dropped++
			continue
		}
		if c.child == "" {
			c.child = obs.Correlation.Child
		} else if obs.Correlation.Child != c.child {
			c.dropped++
			continue
		}
		switch obs.Event {
		case "switch_attempt":
			if c.switchPos < 0 && obs.Result == "ok" {
				c.switchPos = i
			}
		case "model_selected":
			if c.selectedPos < 0 && obs.Model != "" {
				c.selectedPos = i
			}
		case "child_context":
			if c.contextPos < 0 && obs.Model != "" {
				c.contextPos = i
				c.contextID = obs.Model
			}
		case "child_done":
			// No effect on the resolution claim; the end parser reads it.
		}
	}
}

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
// ParseChildEnd feeds `downshift end opencode`. Usage stays nil on purpose —
// see the package comment: the token seam is NOT PROVEN, and a nil func means
// unobserved, never a measured zero. EndLine stays nil: this harness has no
// verified way to show the child-end line.
func Port() hookport.Port {
	return hookport.Port{ID: harnessID, Honor: Honor, ParseChildEnd: ParseChildEnd}
}
