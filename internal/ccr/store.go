// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package ccr keeps the raw output of a compacted command on disk so it can
// be retrieved later by id (compress, cache, retrieve).
package ccr

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/tiagovilasboas/downshift-ai/internal/lazyre"
	"github.com/tiagovilasboas/downshift-ai/internal/paths"
)

var (
	ErrNotFound  = errors.New("ccr: output not found")
	ErrExpired   = errors.New("ccr: output expired")
	ErrTooLarge  = errors.New("ccr: output too large to keep")
	ErrInvalidID = errors.New("ccr: invalid id")
)

var reID = lazyre.New(`^[0-9a-f]{12}$`)

// ValidID reports whether id has the shape Put produces.
func ValidID(id string) bool { return reID.MatchString(id) }

// Store is a directory of retained outputs, one <id>.txt each.
type Store struct {
	Dir                          string
	TTL                          time.Duration
	MaxEntryBytes, MaxTotalBytes int64
	Now                          func() time.Time
}

// Default returns a Store under the Downshift state directory.
func Default() (Store, error) {
	dir, err := paths.Join("ccr")
	if err != nil {
		return Store{}, err
	}
	return Store{Dir: dir, TTL: 24 * time.Hour, MaxEntryBytes: 8 << 20, MaxTotalBytes: 64 << 20, Now: time.Now}, nil
}

func (s Store) now() time.Time {
	if s.Now == nil {
		return time.Now()
	}
	return s.Now()
}

func (s Store) path(id string) string { return filepath.Join(s.Dir, id+".txt") }

// Put stores raw with secret lines withheld and returns its id.
func (s Store) Put(raw []byte) (string, error) {
	if s.MaxEntryBytes > 0 && int64(len(raw)) > s.MaxEntryBytes {
		return "", ErrTooLarge
	}
	clean := redact(raw)
	sum := sha256.Sum256(clean)
	id := hex.EncodeToString(sum[:])[:12]
	if err := paths.PrivateDir(s.Dir); err != nil {
		return "", err
	}
	p := s.path(id)
	now := s.now()
	if s.sameEntry(p, clean) {
		if err := os.Chtimes(p, now, now); err != nil {
			return "", err
		}
	} else if err := s.write(p, clean, now); err != nil {
		return "", err
	}
	s.prune(id, now)
	return id, nil
}

// sameEntry reports whether p is a regular file holding exactly data, so a
// planted, colliding or truncated entry is rewritten instead of reused.
func (s Store) sameEntry(p string, data []byte) bool {
	info, err := paths.RegularFile(p)
	if err != nil || info.Size() != int64(len(data)) {
		return false
	}
	got, err := os.ReadFile(p)
	return err == nil && bytes.Equal(got, data)
}

func (s Store) write(p string, data []byte, now time.Time) error {
	tmp, err := os.CreateTemp(s.Dir, ".put-*.tmp")
	if err != nil {
		return err
	}
	name := tmp.Name()
	_, werr := tmp.Write(data)
	cerr := tmp.Close()
	if werr == nil {
		werr = cerr
	}
	if werr == nil {
		werr = os.Chmod(name, paths.FilePerm)
	}
	if werr == nil {
		werr = os.Rename(name, p)
	}
	if werr != nil {
		_ = os.Remove(name)
		return werr
	}
	return os.Chtimes(p, now, now)
}

// Get returns the stored output for id.
func (s Store) Get(id string) ([]byte, error) {
	if !ValidID(id) {
		return nil, ErrInvalidID
	}
	p := s.path(id)
	info, err := paths.RegularFile(p)
	if errors.Is(err, os.ErrNotExist) || errors.Is(err, paths.ErrNotRegular) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if s.TTL > 0 && s.now().Sub(info.ModTime()) > s.TTL {
		_ = os.Remove(p)
		return nil, ErrExpired
	}
	return os.ReadFile(p)
}

// staleTemp is how old a .put-*.tmp file must be before prune removes it.
const staleTemp = time.Minute

type entry struct {
	path  string
	size  int64
	mtime time.Time
}

// prune drops expired entries, then the oldest while the total is over budget.
// It never removes keep. Best effort: errors are ignored.
func (s Store) prune(keep string, now time.Time) {
	files, err := os.ReadDir(s.Dir)
	if err != nil {
		return
	}
	var live []entry
	var total int64
	for _, f := range files {
		name := f.Name()
		info, err := f.Info()
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		p := filepath.Join(s.Dir, name)
		if strings.HasPrefix(name, ".put-") && now.Sub(info.ModTime()) > staleTemp {
			_ = os.Remove(p) // left behind by a crash between CreateTemp and Rename
			continue
		}
		if !strings.HasSuffix(name, ".txt") {
			continue
		}
		if name != keep+".txt" && s.TTL > 0 && now.Sub(info.ModTime()) > s.TTL {
			_ = os.Remove(p)
			continue
		}
		live = append(live, entry{p, info.Size(), info.ModTime()})
		total += info.Size()
	}
	if s.MaxTotalBytes <= 0 {
		return
	}
	sort.Slice(live, func(i, j int) bool { return live[i].mtime.Before(live[j].mtime) })
	for _, e := range live {
		if total <= s.MaxTotalBytes {
			return
		}
		if filepath.Base(e.path) == keep+".txt" {
			continue
		}
		if os.Remove(e.path) == nil {
			total -= e.size
		}
	}
}
