// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import "github.com/tiagovilasboas/downshift/internal/execwrap"
import "github.com/tiagovilasboas/downshift/internal/telemetry"

// compactorScopeAll says the compactor block totals the whole exec log, every
// session and every run, attributed or not (owner decision 2026-10-21). The
// routing figures beside it stay per session.
const compactorScopeAll = "all"

// compactorPanel is what the Compactor did across the whole exec log. Bytes
// are measured; the token figure is bytes / 4, an estimate, and is never added
// to the tokens the panel reports for model routing (different units,
// different certainty).
type compactorPanel struct {
	Scope                string `json:"scope"`
	Commands             int64  `json:"commands"`
	Compacted            int64  `json:"compacted"`
	RawBytes             int64  `json:"raw_bytes"`
	SavedBytes           int64  `json:"saved_bytes"`
	EstimatedTokensSaved int64  `json:"estimated_tokens_saved"`
	TokenEstimate        string `json:"token_estimate"`
}

// panelOut is the `session --panel` JSON: the routing view as before plus, when
// the exec log has any run, the compactor block.
type panelOut struct {
	telemetry.PanelView
	Compactor *compactorPanel `json:"compactor,omitempty"`
}

// panelWithCompactor adds the Compactor totals of the whole exec log. It does
// not depend on the session: a run with no session hash counts too. No log or
// no run means no block, never zeros.
func panelWithCompactor(v telemetry.PanelView) panelOut {
	out := panelOut{PanelView: v}
	path, err := execwrap.EventsPath()
	if err != nil {
		return out
	}
	s, err := execwrap.Summarize(path)
	if err != nil || s.Commands == 0 {
		return out
	}
	out.Compactor = &compactorPanel{
		Scope: compactorScopeAll, Commands: s.Commands, Compacted: s.Compacted, RawBytes: s.RawBytes,
		SavedBytes: s.SavedBytes, EstimatedTokensSaved: s.EstimatedTokensSaved, TokenEstimate: s.TokenEstimate,
	}
	return out
}
