// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package scoreindex keeps a local cache of public capability scores
// (Epoch AI ECI, Artificial Analysis via OpenRouter) for the session models of
// every harness, and registers core.ScoreHook to read it offline.
//
// Downshift by Tiago de Carvalho Vilas Boas
// https://github.com/tiagovilasboas/downshift
package scoreindex

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/tiagovilasboas/downshift/internal/paths"
)

// FileName is the cache file in the state dir. cmd/downshift/scorerefresh.go
// stats this name for the daily trigger.
const (
	FileName     = "scores.json"
	lockName     = "scores-fetch.lock"
	lockStale    = 10 * time.Minute
	fileVersion  = 1
	EnvSwitch    = "DOWNSHIFT_SCORES"
	envSwitchOff = "off"
)

// SourceTimeout bounds one source request.
const SourceTimeout = 10 * time.Second

var (
	// ErrDisabled: DOWNSHIFT_SCORES=off turns every network call into a no-op.
	ErrDisabled = errors.New("scores disabled (DOWNSHIFT_SCORES=off)")
	// ErrLocked: another refresh holds the lock.
	ErrLocked = errors.New("another refresh is running")
)

// Disabled reports DOWNSHIFT_SCORES=off.
func Disabled() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv(EnvSwitch)), envSwitchOff)
}

// SourceInfo is the per-source state kept in the cache.
type SourceInfo struct {
	Name        string    `json:"name"`
	URL         string    `json:"url"`
	License     string    `json:"license"`
	Attribution string    `json:"attribution"`
	Fetched     time.Time `json:"fetched"`
	ETag        string    `json:"etag,omitempty"`
	Rows        int       `json:"rows"`
	// Status is ok, not-modified, or error: <reason> (the rows are then the
	// previous ones).
	Status string `json:"status"`
}

// File is scores.json. Rows hold every source side by side; only the rows of
// SelectedSource are served to the router, because scales differ.
type File struct {
	Version        int          `json:"version"`
	Updated        time.Time    `json:"updated"`
	SelectedSource string       `json:"selected_source"`
	Sources        []SourceInfo `json:"sources"`
	Rows           []Row        `json:"rows"`
}

// Path is the cache location.
func Path() (string, error) { return paths.Join(FileName) }

// Load reads a cache file. A missing or unreadable file returns an error.
func Load(path string) (*File, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var f File
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, err
	}
	if f.Version != fileVersion {
		return nil, fmt.Errorf("scores cache version %d not supported", f.Version)
	}
	return &f, nil
}

// writeAtomic writes through a temp file in the same directory and renames it.
func writeAtomic(path string, f *File) error {
	data, err := json.MarshalIndent(f, "", " ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".scores-*.tmp")
	if err != nil {
		return err
	}
	name := tmp.Name()
	ok := false
	defer func() {
		if !ok {
			tmp.Close()
			os.Remove(name)
		}
	}()
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(name, 0o600); err != nil {
		return err
	}
	if err := os.Rename(name, path); err != nil {
		return err
	}
	ok = true
	return nil
}

// acquireLock takes the fetch lock exclusively. A lock older than lockStale is
// abandoned and replaced once.
func acquireLock(path string, now time.Time) (release func(), err error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	for attempt := 0; attempt < 2; attempt++ {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			fmt.Fprintf(f, "%d\n", os.Getpid())
			f.Close()
			return func() { os.Remove(path) }, nil
		}
		if !os.IsExist(err) {
			return nil, err
		}
		fi, serr := os.Stat(path)
		if serr != nil || now.Sub(fi.ModTime()) < lockStale {
			return nil, ErrLocked
		}
		if os.Remove(path) != nil {
			return nil, ErrLocked
		}
	}
	return nil, ErrLocked
}

// Options configure Refresh.
type Options struct {
	// Path of scores.json. Empty means the state dir.
	Path string
	// Sources in priority order. Empty means DefaultSources.
	Sources []Source
	Client  *http.Client
	// Sessions is harness -> enabled session model ids, resolved by the caller
	// as the hooks do. It decides which source covers the most.
	Sessions map[string][]string
	Version  string
	Now      func() time.Time
}

// Report is what a refresh did, per source.
type Report struct {
	File    *File
	Results []SourceInfo
}

// Refresh fetches every source, merges with the previous cache and writes
// scores.json atomically. A failed or empty source keeps its previous rows. If
// no source succeeds the file is not touched and an error is returned.
func Refresh(ctx context.Context, o Options) (*Report, error) {
	if Disabled() {
		return nil, ErrDisabled
	}
	now := o.Now
	if now == nil {
		now = time.Now
	}
	path := o.Path
	if path == "" {
		p, err := Path()
		if err != nil {
			return nil, err
		}
		path = p
	}
	release, err := acquireLock(filepath.Join(filepath.Dir(path), lockName), now())
	if err != nil {
		return nil, err
	}
	defer release()

	prev, _ := Load(path)
	sources := o.Sources
	if sources == nil {
		etag := ""
		if prev != nil {
			if si, ok := prev.source(SourceEpoch); ok && si.Rows > 0 {
				etag = si.ETag
			}
		}
		sources = DefaultSources(nil, etag, o.Version)
	}

	stamp := now().UTC()
	next := &File{Version: fileVersion, Updated: stamp}
	succeeded := 0
	for _, s := range sources {
		name := s.Name()
		license, url, attr := Meta(name)
		info := SourceInfo{Name: name, URL: url, License: license, Attribution: attr}
		var oldRows []Row
		if prev != nil {
			if pi, ok := prev.source(name); ok {
				info = pi
				info.URL, info.License, info.Attribution = url, license, attr
				oldRows = prev.rowsOf(name)
			}
		}

		sctx, cancel := context.WithTimeout(ctx, SourceTimeout)
		rows, ferr := s.Fetch(sctx, o.Client)
		cancel()

		switch {
		case ferr == nil && len(rows) > 0:
			succeeded++
			for i := range rows {
				rows[i].Source, rows[i].Fetched = name, stamp
			}
			info.Fetched, info.Rows, info.Status = stamp, len(rows), "ok"
			if et, ok := s.(ETagger); ok {
				info.ETag = et.ETag()
			}
			next.Rows = append(next.Rows, rows...)
		case errors.Is(ferr, ErrNotModified) && len(oldRows) > 0:
			succeeded++
			info.Fetched, info.Rows, info.Status = stamp, len(oldRows), "not-modified"
			if et, ok := s.(ETagger); ok && et.ETag() != "" {
				info.ETag = et.ETag()
			}
			next.Rows = append(next.Rows, oldRows...)
		default:
			reason := "empty response"
			if ferr != nil {
				reason = ferr.Error()
			}
			info.Status = "error: " + reason
			info.Rows = len(oldRows)
			next.Rows = append(next.Rows, oldRows...)
		}
		next.Sources = append(next.Sources, info)
	}
	if succeeded == 0 || len(next.Rows) == 0 {
		return &Report{Results: next.Sources}, errors.New("no source returned scores; cache left unchanged")
	}
	next.SelectedSource = SelectSource(next, o.Sessions)
	if err := writeAtomic(path, next); err != nil {
		return &Report{Results: next.Sources}, err
	}
	return &Report{File: next, Results: next.Sources}, nil
}

func (f *File) source(name string) (SourceInfo, bool) {
	for _, s := range f.Sources {
		if s.Name == name {
			return s, true
		}
	}
	return SourceInfo{}, false
}

func (f *File) rowsOf(name string) []Row {
	var out []Row
	for _, r := range f.Rows {
		if r.Source == name {
			out = append(out, r)
		}
	}
	return out
}

func sessionKey(harness, id string) string { return harness + "\x00" + id }

// Coverage counts the distinct session models of sessions that source name has
// a row for, and the total distinct session models.
func (f *File) Coverage(name string, sessions map[string][]string) (covered, total int) {
	have := map[string]bool{}
	for _, r := range f.Rows {
		if r.Source == name {
			have[sessionKey(r.Harness, r.ID)] = true
		}
	}
	for _, h := range sortedKeys(sessions) {
		seen := map[string]bool{}
		for _, id := range sessions[h] {
			if id == "" || id == "inherit" || seen[id] {
				continue
			}
			seen[id] = true
			total++
			if have[sessionKey(h, id)] {
				covered++
			}
		}
	}
	return covered, total
}

// SelectSource picks the one source whose scores the router may compare: the
// source that covers the most enabled session models, the highest-priority one
// on a tie. Sources without rows are skipped. With no session ids at all every
// coverage is zero, so the highest-priority source with rows wins.
func SelectSource(f *File, sessions map[string][]string) string {
	best, bestCov := "", -1
	for _, s := range f.Sources { // file order is priority order
		if len(f.rowsOf(s.Name)) == 0 {
			continue
		}
		cov, _ := f.Coverage(s.Name, sessions)
		if cov > bestCov {
			best, bestCov = s.Name, cov
		}
	}
	return best
}

func sortedKeys(m map[string][]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
