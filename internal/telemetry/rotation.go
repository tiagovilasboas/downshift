// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// RotationPolicy controls when and how event logs are rotated.
type RotationPolicy struct {
	// MaxSizeMB is the maximum size of a log file before rotation (0 = no rotation).
	MaxSizeMB int64
	// MaxAgeDays is the maximum age of a log file before rotation (0 = no rotation).
	MaxAgeDays int
	// MaxBackups is the maximum number of rotated backups to keep (0 = keep all).
	MaxBackups int
}

// DefaultRotationPolicy returns a reasonable rotation policy:
// - Rotate when log exceeds 100 MB
// - Keep 10 most recent backups
// - No age-based rotation
func DefaultRotationPolicy() RotationPolicy {
	return RotationPolicy{
		MaxSizeMB:  100,
		MaxBackups: 10,
	}
}

// RotationPolicyFromEnv returns the effective rotation policy.
// DOWNSHIFT_LOG_MAX_MB (default 100) and DOWNSHIFT_LOG_MAX_BACKUPS (default 10)
// override sizes; set max MB to 0 to disable rotation.
func RotationPolicyFromEnv() RotationPolicy {
	p := DefaultRotationPolicy()
	if v := os.Getenv("DOWNSHIFT_LOG_MAX_MB"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			p.MaxSizeMB = n
		}
	}
	if v := os.Getenv("DOWNSHIFT_LOG_MAX_BACKUPS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			p.MaxBackups = n
		}
	}
	return p
}

// MaybeRotate checks if the log file at path needs rotation and performs it.
// Rotation is triggered when the file exceeds MaxSizeMB or MaxAgeDays.
// Old backups beyond MaxBackups are deleted.
// Returns the new log path (same as input, or the rotated file path).
func (p RotationPolicy) MaybeRotate(path string) (string, error) {
	if p.MaxSizeMB == 0 && p.MaxAgeDays == 0 {
		// No rotation policy configured.
		return path, nil
	}

	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist yet; no rotation needed.
			return path, nil
		}
		return path, err
	}

	shouldRotate := false

	// Check size.
	if p.MaxSizeMB > 0 && info.Size() > p.MaxSizeMB*1024*1024 {
		shouldRotate = true
	}

	// Check age.
	if p.MaxAgeDays > 0 && time.Since(info.ModTime()) > time.Duration(p.MaxAgeDays)*24*time.Hour {
		shouldRotate = true
	}

	if !shouldRotate {
		return path, nil
	}

	// Rotate: rename current file with timestamp suffix.
	rotatedPath := rotateFile(path)
	if err := os.Rename(path, rotatedPath); err != nil {
		return path, fmt.Errorf("failed to rotate log: %w", err)
	}

	// Clean up old backups.
	if p.MaxBackups > 0 {
		if err := cleanupOldBackups(path, p.MaxBackups); err != nil {
			// Log cleanup errors but don't fail the rotation.
			// (in production, might log this with the observability system)
		}
	}

	return path, nil
}

// rotateFile returns a rotated filename with a timestamp suffix.
// Example: events.jsonl → events.jsonl.2026-10-01T09-44-00
func rotateFile(path string) string {
	now := time.Now().Format("2006-01-02T15-04-05")
	return path + "." + now
}

// cleanupOldBackups deletes rotated backups older than maxBackups.
// Matching files are named path + "." + ISO8601-like timestamp.
func cleanupOldBackups(logPath string, maxBackups int) error {
	dir := filepath.Dir(logPath)
	base := filepath.Base(logPath)

	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	var backups []os.DirEntry
	for _, e := range entries {
		// The lock file shares the prefix but is not a backup.
		if !e.IsDir() && strings.HasPrefix(e.Name(), base+".") && e.Name() != base+".lock" {
			backups = append(backups, e)
		}
	}

	if len(backups) <= maxBackups {
		return nil // Nothing to clean up.
	}

	// Sort by name (timestamp order); delete oldest ones.
	sort.Slice(backups, func(i, j int) bool {
		return backups[i].Name() < backups[j].Name()
	})

	toDelete := len(backups) - maxBackups
	for i := 0; i < toDelete; i++ {
		oldPath := filepath.Join(dir, backups[i].Name())
		if err := os.Remove(oldPath); err != nil {
			// Best-effort cleanup; don't fail if one deletion fails.
		}
	}

	return nil
}
