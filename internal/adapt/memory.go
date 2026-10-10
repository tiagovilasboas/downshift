// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package adapt applies local tier memory learned from prompt-free routing feedback.
// Missing or empty memory is a no-op (fail-open).
package adapt

const fileVersion = 1

// Memory holds per-shape tier hit/miss sets built from reviewed outcomes.
type Memory struct {
	Version int                    `json:"version"`
	Shapes  map[string]ShapeRecord `json:"shapes"`
}

// ShapeRecord aggregates successes and failures per tier for one dominant feature.
type ShapeRecord struct {
	Hits []tierLabel `json:"hits,omitempty"`
	Miss []tierLabel `json:"misses,omitempty"`
}

type tierLabel string

func (m *Memory) empty() bool {
	return m == nil || len(m.Shapes) == 0
}

func (s ShapeRecord) hitSet() map[Tier]bool {
	return tierSet(s.Hits)
}

func (s ShapeRecord) missSet() map[Tier]bool {
	return tierSet(s.Miss)
}

func tierSet(labels []tierLabel) map[Tier]bool {
	out := make(map[Tier]bool, len(labels))
	for _, l := range labels {
		if t, ok := labelToTier(l); ok {
			out[t] = true
		}
	}
	return out
}

func addTier(list []tierLabel, t Tier) []tierLabel {
	l := tierToLabel(t)
	for _, existing := range list {
		if existing == l {
			return list
		}
	}
	return append(list, l)
}

// frozen is true when the shape has at least one hit and zero misses.
func (s ShapeRecord) frozen() bool {
	return len(s.Hits) > 0 && len(s.Miss) == 0
}

// FeedbackEvent is a prompt-free routing outcome used to rebuild memory.
type FeedbackEvent struct {
	Shape        string
	SelectedTier Tier
	Success      bool
	Retry        bool
	Failed       bool
}

// BuildMemory aggregates feedback into shape records. Unknown outcomes are omitted.
func BuildMemory(events []FeedbackEvent) Memory {
	mem := Memory{Version: fileVersion, Shapes: make(map[string]ShapeRecord)}
	for _, e := range events {
		if e.Shape == "" {
			continue
		}
		rec := mem.Shapes[e.Shape]
		switch {
		case e.Success:
			rec.Hits = addTier(rec.Hits, e.SelectedTier)
		case e.Retry, e.Failed:
			rec.Miss = addTier(rec.Miss, e.SelectedTier)
		default:
			continue
		}
		mem.Shapes[e.Shape] = rec
	}
	return mem
}
