// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package discovery

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"time"
)

// DefaultRefreshRetry is how long a refresh marker holds. It stops a second
// hook from starting the same round while the first is running, and after a
// failed round it sets the retry rate, so a harness with no working source is
// not probed by every spawn.
const DefaultRefreshRetry = 5 * time.Minute

// refreshMarkerPattern limits marker names to safe harness ids, so a harness
// id can never name a path outside the cache directory.
var refreshMarkerPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)

// Refresher decides whether one harness needs a background discovery round and
// starts it through Spawn. It runs no source itself. The caller's Spawn starts
// the round in a detached process; it must return without waiting for it.
//
// Every harness with a listing source uses the same rule. The hook calls
// Trigger after it has answered, and the next spawn reads whatever the round
// wrote. A harness without a source never triggers.
type Refresher struct {
	// CachePath is discovered.json. Its directory holds the refresh markers.
	CachePath string
	// Sources is the registry. Only harnesses that have a source refresh.
	Sources []Source
	// TTL is the age after which an entry is stale. Zero means entries never
	// expire, so only a missing entry triggers a round.
	TTL time.Duration
	// Retry is how long a marker holds. Zero means DefaultRefreshRetry.
	Retry time.Duration
	// Now is the clock. Zero means time.Now.
	Now func() time.Time
	// Spawn starts the round for harness detached. A non-nil error keeps the
	// marker, so the same failure is not retried on every hook.
	Spawn func(harness string) error
}

// Trigger starts one round for harness when its entry is missing or older than
// TTL, and no other round for it is in flight or recently failed. It reports
// whether it called Spawn. It never runs a source in the caller's goroutine.
func (r Refresher) Trigger(harness string) (bool, error) {
	if !refreshMarkerPattern.MatchString(harness) || !r.hasSource(harness) || !r.stale(harness) {
		return false, nil
	}
	acquired, err := r.acquire(harness)
	if err != nil || !acquired {
		return false, err
	}
	if r.Spawn == nil {
		return false, errors.New("discovery refresh: no spawn function")
	}
	if err := r.Spawn(harness); err != nil {
		return false, fmt.Errorf("discovery refresh %s: %w", harness, err)
	}
	return true, nil
}

func (r Refresher) now() time.Time {
	if r.Now != nil {
		return r.Now()
	}
	return time.Now()
}

func (r Refresher) retry() time.Duration {
	if r.Retry > 0 {
		return r.Retry
	}
	return DefaultRefreshRetry
}

func (r Refresher) hasSource(harness string) bool {
	for _, s := range r.Sources {
		if s.Harness() == harness {
			return true
		}
	}
	return false
}

// stale reports a missing entry, or one older than TTL. A future-dated entry
// is not stale here; LoadDiscoveredSession already rejects it on the hook path.
func (r Refresher) stale(harness string) bool {
	e, ok := Read(r.CachePath).Harnesses[harness]
	if !ok {
		return true
	}
	if r.TTL == 0 {
		return false
	}
	return r.now().Sub(e.FetchedAt) > r.TTL
}

// acquire creates the marker exclusively, so concurrent triggers race on the
// filesystem and only one wins. A marker older than the retry window is
// abandoned and replaced once.
func (r Refresher) acquire(harness string) (bool, error) {
	dir := filepath.Dir(r.CachePath)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return false, err
	}
	path := filepath.Join(dir, "discovery-"+harness+".refresh")
	for attempt := 0; attempt < 2; attempt++ {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			_, werr := fmt.Fprintf(f, "%s\n", r.now().UTC().Format(time.RFC3339))
			cerr := f.Close()
			return werr == nil && cerr == nil, nil
		}
		if !errors.Is(err, fs.ErrExist) {
			return false, err
		}
		fi, serr := os.Stat(path)
		if errors.Is(serr, fs.ErrNotExist) {
			continue // released between the two calls; try again
		}
		if serr != nil {
			return false, serr
		}
		if r.now().Sub(fi.ModTime()) < r.retry() {
			return false, nil
		}
		if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return false, err
		}
	}
	return false, nil
}
