// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package session

import (
	"encoding/json"
	"os"
	"time"

	"github.com/tiagovilasboas/downshift/internal/paths"
)

// LoadDiscoveredSession reads the models `downshift models discover` found for
// harness. It only reads a local file: no network, no process, so it is safe
// on the hook hot path. An absent, invalid, empty or expired entry is an
// unknown session, which never rewrites.
//
// The ids are already ranked least to most capable by the discovery step, one
// per family; this layer does not rank or look at the catalog.
func LoadDiscoveredSession(harness string, now time.Time) SessionList {
	if harness == "" || discoveryOff() {
		return UnknownSession()
	}
	path := discoveredEnv()
	if path == "" {
		p, err := paths.Join("discovered.json")
		if err != nil {
			return UnknownSession()
		}
		path = p
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return UnknownSession()
	}
	var f struct {
		Harnesses map[string]struct {
			FetchedAt time.Time      `json:"fetched_at"`
			Ordered   []string       `json:"ordered"`
			Metadata  []SessionModel `json:"model_metadata"`
		} `json:"harnesses"`
	}
	if json.Unmarshal(data, &f) != nil {
		return UnknownSession()
	}
	e, ok := f.Harnesses[harness]
	if !ok || len(e.Ordered) == 0 {
		return UnknownSession()
	}
	ttl, invalid := discoveryTTL()
	if invalid {
		return UnknownSession()
	}
	// An entry dated in the future (clock skew or a tampered file) would never
	// age out, so anything beyond a small skew allowance is treated as expired.
	age := now.Sub(e.FetchedAt)
	if age < -5*time.Minute || (ttl > 0 && age > ttl) {
		return UnknownSession()
	}
	list := KnownSession(e.Ordered)
	list.Metadata = e.Metadata
	return list
}
