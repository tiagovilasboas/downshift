// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package training

import (
	"github.com/tiagovilasboas/downshift/internal/adapt"
)

// RefreshAdaptMemory rebuilds adapt-memory.json from reviewed events in the store.
func RefreshAdaptMemory(store *EventStore) error {
	events, err := store.Load()
	if err != nil {
		return err
	}
	var feedback []adapt.FeedbackEvent
	for _, e := range events {
		if e.Outcome == nil || validateOutcome(*e.Outcome) != nil {
			continue
		}
		key := adapt.MemoryKeyForFeatures(e.ComplexityClass, e.Features, "")
		if key == "" {
			continue
		}
		fb := adapt.FeedbackEvent{
			Shape:        key,
			SelectedTier: adapt.Tier(e.SelectedTier),
			Success:      e.Outcome.Success,
			Retry:        e.Outcome.Retry,
			Failed:       e.Outcome.Failed,
		}
		feedback = append(feedback, fb)
	}
	mem := adapt.BuildMemory(feedback)
	if len(mem.Shapes) == 0 {
		return nil
	}
	return adapt.SaveDefault(mem)
}
