// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/tiagovilasboas/downshift/internal/paths"
)

// PauseTTL is how long a pause marker stays in force. A forgotten pause must
// not switch routing off for good.
const PauseTTL = 12 * time.Hour

// sessionHashPattern is the shape HashSessionID returns. Only a value of this
// shape names a marker file, so a hostile id can never build a path.
var sessionHashPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)

// ValidSessionHash reports whether h is a session hash HashSessionID produced.
func ValidSessionHash(h string) bool { return sessionHashPattern.MatchString(h) }

// pausePath is the marker file of one session: an empty file named by the
// hashed session id. Its modification time starts the TTL.
func pausePath(sessionHash string) (string, error) {
	if !ValidSessionHash(sessionHash) {
		return "", os.ErrInvalid
	}
	return paths.Join("paused", sessionHash)
}

// SessionPaused reports whether routing is paused for the session. It costs one
// stat and fails open: an unknown session, a missing or expired marker, or any
// error means not paused. It never writes.
func SessionPaused(sessionHash string) bool {
	return sessionPausedAt(sessionHash, time.Now())
}

func sessionPausedAt(sessionHash string, now time.Time) bool {
	p, err := pausePath(sessionHash)
	if err != nil {
		return false
	}
	fi, err := os.Stat(p)
	if err != nil {
		return false
	}
	return now.Sub(fi.ModTime()) < PauseTTL
}

// PauseSession writes (or refreshes) the pause marker for the session.
func PauseSession(sessionHash string) error {
	p, err := pausePath(sessionHash)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	if err := os.WriteFile(p, nil, 0o600); err != nil {
		return err
	}
	now := time.Now()
	return os.Chtimes(p, now, now) // refresh the TTL when the file already existed
}

// ResumeSession removes the pause marker. A missing marker is not an error.
func ResumeSession(sessionHash string) error {
	p, err := pausePath(sessionHash)
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
