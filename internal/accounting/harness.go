// SPDX-License-Identifier: Apache-2.0

package accounting

import (
	"github.com/tiagovilasboas/downshift/internal/execwrap"
)

// HarnessMetrics aggregates exec events by harness.
type HarnessMetrics struct {
	// Harness is the harness name ("kirocrew", "claude-code", etc.).
	Harness string `json:"harness"`

	// Commands is the total exec events from this harness.
	Commands int64 `json:"commands"`

	// Compacted is the count of commands whose output was compressed.
	Compacted int64 `json:"compacted"`

	// RawBytes is the total bytes before compression.
	RawBytes int64 `json:"raw_bytes"`

	// SavedBytes is RawBytes minus emitted bytes.
	SavedBytes int64 `json:"saved_bytes"`

	// ByEntry breaks down by entry type (hook vs direct).
	ByEntry map[string]int64 `json:"by_entry,omitempty"`

	// ByFamily breaks down by command family (go_test, gh_cli, etc.).
	ByFamily map[string]int64 `json:"by_family,omitempty"`
}

// HarnessSummary aggregates all exec events by harness.
type HarnessSummary struct {
	// Total is the overall count of events.
	Total int64 `json:"total"`

	// ByHarness is the per-harness breakdown.
	ByHarness []HarnessMetrics `json:"by_harness"`
}

// AggregateByHarness groups exec events by their harness field.
// Events with empty harness are grouped under "unknown".
func AggregateByHarness(events []execwrap.Event) HarnessSummary {
	byHarness := make(map[string]*HarnessMetrics)

	for _, ev := range events {
		harness := ev.Harness
		if harness == "" {
			harness = "unknown"
		}

		m, ok := byHarness[harness]
		if !ok {
			m = &HarnessMetrics{
				Harness:  harness,
				ByEntry:  make(map[string]int64),
				ByFamily: make(map[string]int64),
			}
			byHarness[harness] = m
		}

		m.Commands++
		if ev.Compacted {
			m.Compacted++
		}
		m.RawBytes += ev.RawBytes
		m.SavedBytes += ev.RawBytes - ev.EmittedBytes

		entry := ev.Entry
		if entry == "" {
			entry = "unknown"
		}
		m.ByEntry[entry]++

		family := string(ev.Family)
		if family == "" {
			family = "other"
		}
		m.ByFamily[family]++
	}

	// Convert to slice, sorted by commands desc.
	result := HarnessSummary{
		Total:     int64(len(events)),
		ByHarness: make([]HarnessMetrics, 0, len(byHarness)),
	}

	for _, m := range byHarness {
		result.ByHarness = append(result.ByHarness, *m)
	}

	// Sort by commands descending.
	sortByCommands(result.ByHarness)

	return result
}

func sortByCommands(metrics []HarnessMetrics) {
	// Simple insertion sort (usually small N).
	for i := 1; i < len(metrics); i++ {
		j := i
		for j > 0 && metrics[j].Commands > metrics[j-1].Commands {
			metrics[j], metrics[j-1] = metrics[j-1], metrics[j]
			j--
		}
	}
}
