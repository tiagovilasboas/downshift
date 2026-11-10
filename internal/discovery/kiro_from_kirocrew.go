// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package discovery

import (
	"context"
	"errors"
	"time"
)

// KiroFromKiroCrew mirrors the KiroCrew model list to Kiro IDE. Both harnesses
// share the same AWS/Kiro account, so the available models are identical. This
// source enables `downshift models discover` to populate Kiro IDE automatically
// without requiring a separate CLI or API call.
//
// The source reads the existing KiroCrew discovery cache entry. If KiroCrew has
// no cache or the cache is stale, it returns an error and the caller keeps the
// previous Kiro entry (if any). Run `downshift models discover --harness kirocrew`
// first when using a new account or after model changes.
type KiroFromKiroCrew struct {
	// CachePath overrides the discovery cache path for testing.
	CachePath string
	// MaxAge is the oldest KiroCrew entry to accept. Zero means 7 days.
	MaxAge time.Duration
	// Now returns the current time. Nil means time.Now.
	Now func() time.Time
}

func (KiroFromKiroCrew) Harness() string { return "kiro" }
func (KiroFromKiroCrew) Name() string    { return "kirocrew discovery (same account)" }

var errKiroCrewCacheStale = errors.New("kiro: kirocrew discovery cache is stale or missing; run: downshift models discover --harness kirocrew")

func (k KiroFromKiroCrew) Discover(ctx context.Context) ([]Model, error) {
	path := k.CachePath
	if path == "" {
		var err error
		path, err = Path()
		if err != nil {
			return nil, err
		}
	}
	cache := Read(path)
	entry, ok := cache.Harnesses["kirocrew"]
	if !ok || len(entry.Models) == 0 {
		return nil, errKiroCrewCacheStale
	}

	// Check freshness: default 7 days, configurable for tests.
	maxAge := k.MaxAge
	if maxAge == 0 {
		maxAge = 7 * 24 * time.Hour
	}
	now := k.Now
	if now == nil {
		now = time.Now
	}
	age := now().Sub(entry.FetchedAt)
	if age < 0 || age > maxAge {
		return nil, errKiroCrewCacheStale
	}

	// Copy models, preserving order but not KiroCrew-specific metadata.
	out := make([]Model, 0, len(entry.Models))
	for _, m := range entry.Models {
		if m.ID == "" {
			continue
		}
		out = append(out, Model{ID: m.ID, Display: m.Display})
	}
	if len(out) == 0 {
		return nil, errKiroCrewCacheStale
	}
	return out, nil
}
