// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package discovery asks each harness which models the current account can
// use, instead of trusting a compiled-in list. It runs out of band (the
// `downshift models discover` command), never in a hook: hooks read the cache
// it writes, so the routing hot path stays offline and deterministic.
//
// Precedence at routing time (see core.ResolveSessionForID):
// hook payload, then this cache, then the operator's session-models.json.
// quota.<harness> in the operator file is not read. Discovery reports model
// availability, never remaining credit; live quota is a separate input.
// The embedded catalog is metadata (tier, cost, family) used to rank what was
// discovered; it never adds an id the harness did not report.
package discovery

import (
	"context"
	"errors"
	"regexp"
	"time"
)

// Model is one model a harness reported, in the order the source listed it.
type Model struct {
	ID        string    `json:"id"`
	Display   string    `json:"display_name,omitempty"`
	CreatedAt time.Time `json:"created_at,omitempty"`
	// Optional provider capability evidence, never inferred from picker order.
	Tier    string   `json:"tier,omitempty"`
	Efforts []string `json:"efforts,omitempty"`
	// Gateway marks an id from a gateway listing ("vendor/model", not a
	// harness id). It is ranked only through an allowlist row, never by
	// stripping the vendor prefix.
	Gateway bool `json:"gateway,omitempty"`
}

// Source reports the models one harness offers to the current account.
type Source interface {
	// Harness is the catalog harness id the models belong to.
	Harness() string
	// Name identifies the mechanism (file, CLI, API) for reports.
	Name() string
	// Discover returns the offered models. An error means "unknown", never
	// "none": callers keep the previous cache entry.
	Discover(ctx context.Context) ([]Model, error)
}

// Outcome is the result of running one source.
type Outcome struct {
	Source Source
	Models []Model
	Err    error
}

// Run discovers from every source concurrently-safe in sequence (sources are
// cheap local reads or one short call) and returns one outcome per source.
// Each source gets its own timeout so one slow CLI cannot stall the rest.
func Run(ctx context.Context, sources []Source, perSource time.Duration) []Outcome {
	out := make([]Outcome, 0, len(sources))
	for _, s := range sources {
		cctx, cancel := context.WithTimeout(ctx, perSource)
		models, err := s.Discover(cctx)
		cancel()
		models = validModels(models)
		if err == nil && len(models) == 0 {
			err = errors.New("source returned no models")
		}
		out = append(out, Outcome{Source: s, Models: models, Err: err})
	}
	return out
}

// modelIDPattern is the shape every discovered id must have, whatever the
// source: no spaces, control or escape characters, bounded length. Ids that do
// not match are dropped before they reach the cache or the terminal.
var modelIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:/+\[\],=-]{0,127}$`)

func validModels(models []Model) []Model {
	out := models[:0:0]
	for _, m := range models {
		if modelIDPattern.MatchString(m.ID) {
			out = append(out, m)
		}
	}
	return out
}
