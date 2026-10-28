// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package models

import (
	"time"

	"github.com/tiagovilasboas/downshift/internal/discovery"
	"github.com/tiagovilasboas/downshift/internal/session"
)

// evidenceRow is the capability data a real source supplied for one id.
type evidenceRow struct {
	tier    string
	efforts []string
}

// sourceRead is what the first source with models returned.
type sourceRead struct {
	source    string
	ids       []string
	meta      map[string]*evidenceRow
	unranked  []string
	writtenAt time.Time
}

// readSource tries the payload, the discovery cache and session-models.json.
// It only reads local files: no network, no process.
func readSource(harness string, opt Options) (sourceRead, bool) {
	if opt.Payload != nil {
		if s, ok := fromList(SourcePayload, session.KnownSession(*opt.Payload)); ok {
			return s, true
		}
		return sourceRead{}, false
	}
	if l := session.LoadDiscoveredSession(harness, opt.Now); l.Known {
		if s, ok := fromList(SourceDiscovery, l); ok {
			s.writtenAt, s.unranked = discoveryEntry(harness, s.ids)
			return s, true
		}
	}
	if s, ok := fromList(SourceFile, session.LoadUserSession(harness)); ok {
		return s, true
	}
	return sourceRead{}, false
}

func fromList(source string, l session.SessionList) (sourceRead, bool) {
	if !l.Known || len(l.IDs) == 0 {
		return sourceRead{}, false
	}
	s := sourceRead{source: source, ids: l.IDs, meta: map[string]*evidenceRow{}}
	for _, m := range l.Metadata {
		s.meta[m.ID] = &evidenceRow{tier: m.Tier, efforts: m.Efforts}
	}
	return s, true
}

// discoveryEntry reads when the cache entry was written and which reported
// ids the catalog could not rank. Ids already listed are not repeated.
func discoveryEntry(harness string, listed []string) (time.Time, []string) {
	path, err := discovery.Path()
	if err != nil {
		return time.Time{}, nil
	}
	e, ok := discovery.Read(path).Harnesses[harness]
	if !ok {
		return time.Time{}, nil
	}
	have := map[string]bool{}
	for _, id := range listed {
		have[id] = true
	}
	var extra []string
	for _, id := range e.Unranked {
		if id != "" && !have[id] {
			have[id] = true
			extra = append(extra, id)
		}
	}
	return e.FetchedAt, extra
}
