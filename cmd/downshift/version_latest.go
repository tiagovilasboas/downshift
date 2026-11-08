// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// The latest-release lookup behind `doctor --check-latest`. It is the only
// network call doctor can make, it runs only on that flag, and never from a
// hook. Any failure is "unknown", never a failed check.

const (
	latestReleaseURL = "https://api.github.com/repos/tiagovilasboas/downshift-ai/releases/latest"
	latestCacheTTL   = 24 * time.Hour
	latestTimeout    = 2 * time.Second
)

// latestLookup asks the releases API for the newest tag, caching the answer in
// CacheFile for latestCacheTTL. Client and URL are injectable for tests.
type latestLookup struct {
	Client    *http.Client
	URL       string
	CacheFile string
	Now       func() time.Time
}

type latestCache struct {
	Tag       string    `json:"tag"`
	CheckedAt time.Time `json:"checked_at"`
}

func defaultLatestLookup(cacheFile string) latestLookup {
	return latestLookup{Client: &http.Client{Timeout: latestTimeout},
		URL: latestReleaseURL, CacheFile: cacheFile, Now: time.Now}
}

// Latest returns the newest release tag and whether it is known.
func (l latestLookup) Latest() (string, bool) {
	if c, ok := l.readCache(); ok && fresh(l.Now().Sub(c.CheckedAt)) {
		return c.Tag, true
	}
	tag, ok := l.fetch()
	if !ok {
		return "", false
	}
	l.writeCache(latestCache{Tag: tag, CheckedAt: l.Now()})
	return tag, true
}

func (l latestLookup) fetch() (string, bool) {
	req, err := http.NewRequest(http.MethodGet, l.URL, nil)
	if err != nil {
		return "", false
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := l.Client.Do(req)
	if err != nil {
		return "", false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", false
	}
	var doc struct {
		Tag string `json:"tag_name"`
	}
	if json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&doc) != nil || doc.Tag == "" {
		return "", false
	}
	return doc.Tag, true
}

func (l latestLookup) readCache() (latestCache, bool) {
	raw, err := os.ReadFile(l.CacheFile)
	if err != nil {
		return latestCache{}, false
	}
	var c latestCache
	if json.Unmarshal(raw, &c) != nil || c.Tag == "" {
		return latestCache{}, false
	}
	return c, true
}

// writeCache is best effort: a read-only state dir only costs a refetch.
func (l latestLookup) writeCache(c latestCache) {
	raw, err := json.Marshal(c)
	if err != nil {
		return
	}
	if os.MkdirAll(filepath.Dir(l.CacheFile), 0o700) != nil {
		return
	}
	_ = os.WriteFile(l.CacheFile, raw, 0o600)
}

// fresh reports whether a cache entry of this age is still usable. A negative
// age (a clock moved back, or a cache from the future) is not fresh.
func fresh(age time.Duration) bool { return age >= 0 && age < latestCacheTTL }
