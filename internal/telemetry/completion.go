// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import "fmt"

// HarnessCompletion says whether the end of a child spawn is measurable for one
// harness: the harness declares an end event and at least one end record has
// been seen. Without that, "completed" and tokens per completed task cannot be
// computed for the harness, and the report says so instead of showing zeros.
type HarnessCompletion struct {
	Harness string `json:"harness"`
	// EndEvent is HarnessSeam.ChildEndEvent; empty when none is known.
	EndEvent string `json:"end_event,omitempty"`
	// UsageSource is HarnessSeam.UsageSource: payload, transcript or none.
	UsageSource string `json:"usage_source"`
	// EndRecords counts the end records seen for the harness (see IsEndRecord).
	EndRecords int  `json:"end_records"`
	Measurable bool `json:"measurable"`
}

// IsEndRecord reports an event that proves a child ended: a usage record (it is
// written when the child stops) or a child_ended record whose status is an end,
// not a launch.
func IsEndRecord(ev Event) bool {
	switch ev.Outcome {
	case OutcomeUsage:
		return true
	case OutcomeChildEnded:
		return ev.ChildStatus != "" && ev.ChildStatus != ChildLaunched && ev.ChildStatus != ChildUnknown
	}
	return false
}

// HarnessSeam is what telemetry knows about one declared harness: the hook
// event that ends a child and where the child's token usage comes from
// (payload, transcript or UsageNone; empty reads as UsageNone).
type HarnessSeam struct {
	Harness       string
	ChildEndEvent string
	UsageSource   string
}

// UsageNone is the usage source of a harness with no token seam.
const UsageNone = "none"

// DeclaredHarnesses lists every declared harness in a stable order. The
// composition root sets it from the harness capabilities
// (internal/telemetry/fromcore); with nil no harness is declared.
var DeclaredHarnesses []HarnessSeam

// CompletionByHarness lists, in the stable order of DeclaredHarnesses,
// whether completion is measurable for each harness in events.
func CompletionByHarness(events []Event) []HarnessCompletion {
	ends := map[string]int{}
	for _, ev := range events {
		if IsEndRecord(ev) {
			ends[ev.Harness]++
		}
	}
	var out []HarnessCompletion
	for _, n := range DeclaredHarnesses {
		src := n.UsageSource
		if src == "" {
			src = UsageNone
		}
		c := HarnessCompletion{Harness: n.Harness, EndEvent: n.ChildEndEvent, UsageSource: src, EndRecords: ends[n.Harness]}
		c.Measurable = c.EndEvent != "" && c.EndRecords > 0
		out = append(out, c)
	}
	return out
}

// Describe is the one-line text report and doctor print for a harness.
func (c HarnessCompletion) Describe() string {
	switch {
	case c.EndEvent == "":
		return "no end event known; completion is not measurable"
	case c.Measurable:
		return "measurable: " + plural(c.EndRecords, "end record") + " from " + c.EndEvent + ", usage from " + c.UsageSource
	}
	return "not yet measurable: no end record seen from " + c.EndEvent + " (usage from " + c.UsageSource + ")"
}

func plural(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return fmt.Sprintf("%d %ss", n, noun)
}
