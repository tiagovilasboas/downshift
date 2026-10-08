// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"encoding/json"
	"os"
	"time"

	"github.com/tiagovilasboas/downshift/internal/paths"
)

const (
	// EnvDiscovery set to "off" disables the discovered-models layer.
	EnvDiscovery = "DOWNSHIFT_DISCOVERY"
	// EnvDiscoveryTTL is a Go duration; "0" keeps a cache entry forever.
	EnvDiscoveryTTL = "DOWNSHIFT_DISCOVERY_TTL"
	// EnvDiscovered overrides the cache path.
	EnvDiscovered = "DOWNSHIFT_DISCOVERED"

	defaultDiscoveryTTL = 24 * time.Hour
)

// LoadDiscoveredSession reads the models `downshift models discover` found for
// harness. It only reads a local file: no network, no process, so it is safe
// on the hook hot path. An absent, invalid, empty or expired entry is an
// unknown session, which never rewrites.
//
// The ids are already ranked least to most capable by the discovery step, one
// per family; this layer does not rank or look at the catalog.
func LoadDiscoveredSession(harness string, now time.Time) SessionList {
	if harness == "" || os.Getenv(EnvDiscovery) == "off" {
		return UnknownSession()
	}
	path := os.Getenv(EnvDiscovered)
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
			FetchedAt time.Time `json:"fetched_at"`
			Ordered   []string  `json:"ordered"`
		} `json:"harnesses"`
	}
	if json.Unmarshal(data, &f) != nil {
		return UnknownSession()
	}
	e, ok := f.Harnesses[harness]
	if !ok || len(e.Ordered) == 0 {
		return UnknownSession()
	}
	ttl := defaultDiscoveryTTL
	if v := os.Getenv(EnvDiscoveryTTL); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil || d < 0 {
			return UnknownSession()
		}
		ttl = d
	}
	if ttl > 0 && now.Sub(e.FetchedAt) > ttl {
		return UnknownSession()
	}
	return KnownSession(e.Ordered)
}
