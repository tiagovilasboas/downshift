// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package scoreindex

import (
	"os"
	"sync"
	"time"

	"github.com/tiagovilasboas/downshift/internal/core"
)

// The router reads scores through core.ScoreHook, registered here like
// core.TierFloorHook. The lookup is offline: it reads scores.json only, and
// serves the rows of selected_source alone so two scales never meet.
func init() { core.ScoreHook = Lookup }

type snapshot struct {
	path  string
	mtime time.Time
	size  int64
	byKey map[string]float64
}

var (
	snapMu sync.Mutex
	snap   *snapshot
)

// Lookup returns the score of a session model id from the selected source.
// ok is false when scores are off, the cache is missing or invalid, or the id
// has no row in the selected source.
func Lookup(harness, id string) (float64, bool) {
	if Disabled() || harness == "" || id == "" {
		return 0, false
	}
	path, err := Path()
	if err != nil {
		return 0, false
	}
	fi, err := os.Stat(path)
	if err != nil {
		return 0, false
	}
	snapMu.Lock()
	defer snapMu.Unlock()
	if snap == nil || snap.path != path || !snap.mtime.Equal(fi.ModTime()) || snap.size != fi.Size() {
		snap = loadSnapshot(path, fi)
	}
	v, ok := snap.byKey[sessionKey(harness, id)]
	return v, ok
}

func loadSnapshot(path string, fi os.FileInfo) *snapshot {
	s := &snapshot{path: path, mtime: fi.ModTime(), size: fi.Size(), byKey: map[string]float64{}}
	f, err := Load(path)
	if err != nil || f.SelectedSource == "" {
		return s
	}
	for _, r := range f.Rows {
		if r.Source == f.SelectedSource {
			s.byKey[sessionKey(r.Harness, r.ID)] = r.Score
		}
	}
	return s
}
