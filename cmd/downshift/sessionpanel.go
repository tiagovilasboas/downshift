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
// are measured; the token figure is bytes / 4, an estimate. The panel's final
// figure, tokens_saved_estimate, adds it to the session's routing tokens.
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
// the exec log has any run, the compactor block, and the final figure.
//
// TokensSavedEstimate scope: routing tokens saved in THIS session plus the
// Compactor's all-sessions total (the same scopes the tiles had). The panel is
// session-centred, but the Compactor total is also shown when the session is
// unknown, so the sum stays consistent with what the user already saw.
type panelOut struct {
	telemetry.PanelView
	Compactor           *compactorPanel `json:"compactor,omitempty"`
	TokensSavedEstimate int64           `json:"tokens_saved_estimate"`
}

// panelWithCompactor adds the Compactor totals of the whole exec log. It does
// not depend on the session: a run with no session hash counts too. No log or
// no run means no block, never zeros.
func panelWithCompactor(v telemetry.PanelView) panelOut {
	out := panelOut{PanelView: v, TokensSavedEstimate: telemetry.TokensSavedEstimate(v.Tokens, 0)}
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
	out.TokensSavedEstimate = telemetry.TokensSavedEstimate(v.Tokens, s.EstimatedTokensSaved)
	return out
}
