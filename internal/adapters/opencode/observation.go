// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package opencode

import (
	"bytes"
	"encoding/json"
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
