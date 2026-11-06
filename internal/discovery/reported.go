// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package discovery

import (
	"time"

	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/session"
)

// MaxReported caps the ids accepted from one report.
const MaxReported = 64

// Reasons of a ReportResult.
const (
	ReasonWritten      = "written"
	ReasonFreshWins    = "fresh-detection-wins"
	ReasonNoValidIDs   = "no-valid-ids"
	ReasonEmptyHarness = "unknown-harness"
)

// ReportResult says what MergeReported did with a list the orchestrator
// reported through the MCP providers tool.
type ReportResult struct {
	Accepted int
	Written  bool
	Reason   string
}

// MergeReported records model ids reported by the orchestrator as an entry
// with Source "mcp". The data is informative: a fresh entry from a real
// detection is never touched, and the catalog only ranks the ids, it never
// adds one.
func MergeReported(path, harness string, ids []string, now time.Time, cat Catalog) (ReportResult, error) {
	if harness == "" {
		return ReportResult{Reason: ReasonEmptyHarness}, nil
	}
	models := reportedModels(ids)
	if len(models) == 0 {
		return ReportResult{Reason: ReasonNoValidIDs}, nil
	}
	mergeMu.Lock()
	defer mergeMu.Unlock()
	unlock, err := lockCache(path)
	if err != nil {
		return ReportResult{Accepted: len(models)}, err
	}
	defer unlock()
	cur, hasCur := Read(path).Harnesses[harness]
	if hasCur && detectionIsFresh(cur, now) {
		return ReportResult{Accepted: len(models), Reason: ReasonFreshWins}, nil
	}
	union := unionWithBase(reportedBase(harness, cur, hasCur, cat), models)
	e := Entry{Source: session.SourceMCP, FetchedAt: now.UTC(), Models: union, Ranked: Order(harness, union, cat)}
	if err := mergeLocked(path, map[string]Entry{harness: e}); err != nil {
		return ReportResult{Accepted: len(models)}, err
	}
	return ReportResult{Accepted: len(models), Written: true, Reason: ReasonWritten}, nil
}

// reportedBase is the list the harness had the moment the report arrived, which
// the reported ids are added to so a report can only widen the view: the models
// of the existing cache entry (a stale detection or an earlier mcp hint still
// within its life) when there is one, otherwise, for a harness whose session
// comes from the catalog, the catalog stand-in. A harness with no entry and no
// stand-in has no base, so the reported ids stand alone, as before.
func reportedBase(harness string, cur Entry, hasCur bool, cat Catalog) []Model {
	if hasCur && len(cur.Models) > 0 {
		return cur.Models
	}
	if caps, ok := core.CapsFor(harness); ok && caps.SessionFromCatalog {
		return catalogStandIn(harness, cat)
	}
	return nil
}

// catalogStandIn is the session stand-in the model-availability surfaces show
// for a SessionFromCatalog harness: each session candidate the catalog resolves
// to a model that is not explicit-only, distinct, in order. It invents no id
// beyond the candidates the catalog already serves.
func catalogStandIn(harness string, cat Catalog) []Model {
	if cat == nil {
		return nil
	}
	seen := map[string]bool{}
	var out []Model
	for _, name := range cat.SessionCandidates(harness) {
		m, ok := cat.LookupByID(harness, name)
		if !ok || m.ID == "" || seen[m.ID] || cat.IsExplicitOnly(harness, m.ID) {
			continue
		}
		seen[m.ID] = true
		out = append(out, Model{ID: m.ID})
	}
	return out
}

// unionWithBase appends the reported models that base does not already list,
// keeping base's order first and then the new ids in reported order.
func unionWithBase(base, reported []Model) []Model {
	seen := map[string]bool{}
	out := make([]Model, 0, len(base)+len(reported))
	for _, m := range base {
		if m.ID == "" || seen[m.ID] {
			continue
		}
		seen[m.ID] = true
		out = append(out, m)
	}
	for _, m := range reported {
		if m.ID == "" || seen[m.ID] {
			continue
		}
		seen[m.ID] = true
		out = append(out, m)
	}
	return out
}

// reportedModels keeps valid, distinct ids in order, up to MaxReported.
func reportedModels(ids []string) []Model {
	seen := map[string]bool{}
	var out []Model
	for _, id := range ids {
		if len(out) == MaxReported {
			break
		}
		if seen[id] || !modelIDPattern.MatchString(id) {
			continue
		}
		seen[id] = true
		out = append(out, Model{ID: id})
	}
	return out
}

// detectionIsFresh reports a non-MCP entry that still has models and is
// inside the discovery TTL (an invalid TTL makes the cache unknown, so it is
// not fresh).
func detectionIsFresh(e Entry, now time.Time) bool {
	if e.Source == session.SourceMCP || len(e.IDs) == 0 {
		return false
	}
	ttl, invalid := session.DiscoveryTTL()
	if invalid {
		return false
	}
	age := now.Sub(e.FetchedAt)
	return age >= -5*time.Minute && (ttl == 0 || age <= ttl)
}
