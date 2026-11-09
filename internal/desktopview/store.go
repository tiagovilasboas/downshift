// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package desktopview

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/tiagovilasboas/downshift-ai/internal/paths"
)

// Store keeps the last compact view of each desktop read scope, so the next
// read of the same scope can be sent as a delta. One file per scope, named by
// a hash of the scope key (never the key itself: it holds app and window
// names). Files are private (0600) and expire after TTL.
type Store struct {
	Dir      string
	TTL      time.Duration
	MaxBytes int64
	Now      func() time.Time
}

// DefaultTTL is how long a previous view stays a valid base. An agent that
// reads the same window again after this long gets the full view.
const DefaultTTL = 10 * time.Minute

// DefaultMaxBytes caps one stored view. A larger view is sent whole and not
// kept as a base.
const DefaultMaxBytes = 2 << 20

// Default returns a Store under the Downshift state directory.
func Default() (Store, error) {
	dir, err := paths.Join("desktopview")
	if err != nil {
		return Store{}, err
	}
	return Store{Dir: dir, TTL: DefaultTTL, MaxBytes: DefaultMaxBytes, Now: time.Now}, nil
}

func (s Store) now() time.Time {
	if s.Now == nil {
		return time.Now()
	}
	return s.Now()
}

func (s Store) path(key string) string {
	sum := sha256.Sum256([]byte(key))
	return filepath.Join(s.Dir, hex.EncodeToString(sum[:])[:16]+".view")
}

// Load returns the view kept for key, or ok false when there is none, it
// expired, or it is not a regular file.
func (s Store) Load(key string) (view []byte, ok bool) {
	p := s.path(key)
	info, err := paths.RegularFile(p)
	if err != nil {
		return nil, false
	}
	if s.TTL > 0 && s.now().Sub(info.ModTime()) > s.TTL {
		_ = os.Remove(p)
		return nil, false
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return nil, false
	}
	return data, true
}

// ErrTooLarge is returned by Save for a view over MaxBytes.
var ErrTooLarge = errors.New("desktopview: view too large to keep")

// Save keeps view as the base for key, replacing any earlier one.
func (s Store) Save(key string, view []byte) error {
	if s.MaxBytes > 0 && int64(len(view)) > s.MaxBytes {
		_ = os.Remove(s.path(key))
		return ErrTooLarge
	}
	if err := paths.PrivateDir(s.Dir); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(s.Dir, ".put-*.tmp")
	if err != nil {
		return err
	}
	name := tmp.Name()
	_, werr := tmp.Write(view)
	if cerr := tmp.Close(); werr == nil {
		werr = cerr
	}
	if werr == nil {
		werr = os.Chmod(name, paths.FilePerm)
	}
	if werr == nil {
		werr = os.Rename(name, s.path(key))
	}
	if werr != nil {
		_ = os.Remove(name)
		return werr
	}
	now := s.now()
	return os.Chtimes(s.path(key), now, now)
}
