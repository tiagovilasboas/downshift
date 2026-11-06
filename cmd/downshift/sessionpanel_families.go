// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"sort"
	"time"

	"github.com/tiagovilasboas/downshift/internal/accounting"
	"github.com/tiagovilasboas/downshift/internal/execwrap"
)

// compactorFamilyPanel is one command family of the session's Compactor runs.
// Tokens are estimates (bytes / execwrap.BytesPerToken).
type compactorFamilyPanel struct {
	Family        string `json:"family"`
	Commands      int64  `json:"commands"`
	Compacted     int64  `json:"compacted"`
	RawTokens     int64  `json:"raw_tokens"`
	AvoidedTokens int64  `json:"avoided_tokens"`
}

// sessionFamilies maps accounting's per-harness detail of the session scope to
// one list per family, summed across harnesses, largest avoided tokens first,
// then family name. Never nil.
func sessionFamilies(execEvents []execwrap.Event, session accounting.Scope) []compactorFamilyPanel {
	detail := accounting.BuildCompactionDetail(execEvents, nil, session, time.Now())
	byName := map[string]*compactorFamilyPanel{}
	for _, h := range detail.Harnesses {
		for _, f := range h.Families {
			p := byName[f.Family]
			if p == nil {
				p = &compactorFamilyPanel{Family: f.Family}
				byName[f.Family] = p
			}
			p.Commands += f.Commands
			p.Compacted += f.Compacted
			p.RawTokens += f.RawTokens
			p.AvoidedTokens += f.AvoidedTokens
		}
	}
	out := []compactorFamilyPanel{}
	for _, p := range byName {
		out = append(out, *p)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].AvoidedTokens != out[j].AvoidedTokens {
			return out[i].AvoidedTokens > out[j].AvoidedTokens
		}
		return out[i].Family < out[j].Family
	})
	return out
}
