// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package discovery

import (
	"time"

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
	if cur, ok := Read(path).Harnesses[harness]; ok && detectionIsFresh(cur, now) {
		return ReportResult{Accepted: len(models), Reason: ReasonFreshWins}, nil
	}
	e := Entry{Source: session.SourceMCP, FetchedAt: now.UTC(), Models: models, Ranked: Order(harness, models, cat)}
	if err := mergeLocked(path, map[string]Entry{harness: e}); err != nil {
		return ReportResult{Accepted: len(models)}, err
	}
	return ReportResult{Accepted: len(models), Written: true, Reason: ReasonWritten}, nil
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
