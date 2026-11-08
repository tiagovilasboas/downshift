// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import "github.com/tiagovilasboas/downshift-ai/internal/telemetry"

// stampParentModel records the parent session model on a decision event.
// Adapters expose it through the optional ParentModelIdentifier method; one
// that does not (or does not know it) leaves both fields absent. Data only:
// routing has already decided and never reads these fields.
func stampParentModel(event *telemetry.Event, ev any) {
	p, ok := ev.(interface{ ParentModelIdentifier() (string, string) })
	if !ok {
		return
	}
	model, source := p.ParentModelIdentifier()
	event.ParentModel, event.ParentModelSource = telemetry.ParentModelLabel(model, source)
}
