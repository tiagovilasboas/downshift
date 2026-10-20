// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import "github.com/tiagovilasboas/downshift/internal/execwrap"
import "github.com/tiagovilasboas/downshift/internal/telemetry"

// compactorPanel is what the Compactor did in one session. Bytes are measured;
// the token figure is bytes / 4, an estimate, and is never added to the tokens
// the panel reports for model routing (different units, different certainty).
type compactorPanel struct {
	Commands             int64  `json:"commands"`
	Compacted            int64  `json:"compacted"`
	RawBytes             int64  `json:"raw_bytes"`
	SavedBytes           int64  `json:"saved_bytes"`
	EstimatedTokensSaved int64  `json:"estimated_tokens_saved"`
	TokenEstimate        string `json:"token_estimate"`
}

// panelOut is the `session --panel` JSON: the routing view as before plus, when
// the session has wrapped commands, the compactor block.
type panelOut struct {
	telemetry.PanelView
	Compactor *compactorPanel `json:"compactor,omitempty"`
}

// panelWithCompactor adds the session's Compactor totals. No log, no session
// hash or no wrapped command of this session means no block, never zeros.
func panelWithCompactor(v telemetry.PanelView, sessionHash string) panelOut {
	out := panelOut{PanelView: v}
	if sessionHash == "" {
		return out
	}
	path, err := execwrap.EventsPath()
	if err != nil {
		return out
	}
	s, err := execwrap.SummarizeSession(path, sessionHash)
	if err != nil || s.Commands == 0 {
		return out
	}
	out.Compactor = &compactorPanel{
		Commands: s.Commands, Compacted: s.Compacted, RawBytes: s.RawBytes,
		SavedBytes: s.SavedBytes, EstimatedTokensSaved: s.EstimatedTokensSaved, TokenEstimate: s.TokenEstimate,
	}
	return out
}
