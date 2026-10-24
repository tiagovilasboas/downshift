// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"strings"

	"github.com/tiagovilasboas/downshift/internal/core"
)

// canonicalizeModels records requested_model and final_model as catalog ids,
// so analysis by model pair does not split one model across its names (Claude
// Code's "haiku" and "claude-haiku-4-5", KiroCrew's "claude-opus-4.8" and
// "claude-opus-4-8"). Only an exact id or a declared alias is rewritten; the
// value the harness sent moves to requested_model_raw / final_model_raw. A
// family-prefix match is left alone (a version the catalog does not list is
// not that row's model), as is a gateway "provider/model" id, whose slug the
// gateway pricing reads. With no catalog nothing changes.
func canonicalizeModels(ev *Event) {
	ev.FromModel, ev.RequestedModelRaw = canonicalModel(ev.Harness, ev.FromModel, ev.RequestedModelRaw)
	ev.ToModel, ev.FinalModelRaw = canonicalModel(ev.Harness, ev.ToModel, ev.FinalModelRaw)
}

// canonicalModel returns the catalog id for id and the raw value to keep. raw
// already set (an unrecognised requested model) is never overwritten.
func canonicalModel(harness, id, raw string) (string, string) {
	exact, ok := DirectionCatalog.(core.ExactIDResolver)
	if !ok || harness == "" || id == "" || id == "unknown" || strings.Contains(id, "/") {
		return id, raw
	}
	if !exact.IsExactID(harness, id) {
		return id, raw
	}
	m, found := DirectionCatalog.LookupByID(harness, id)
	if !found || m.ID == "" || m.ID == id {
		return id, raw
	}
	if raw == "" {
		raw = RawModelOrInvalid(id)
	}
	return ModelOrUnknown(m.ID), raw
}
