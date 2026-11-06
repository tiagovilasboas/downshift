// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package setup

import (
	"os"
	"path/filepath"
	"strings"
)

// backupsDirName is the folder under the state directory that keeps the undo
// markers (and the backups of files inside harness-loaded folders). It mirrors
// the path below the home directory: <state>/backups/<stamp>/<relative path>.
// Keeping them there leaves a harness plugin or skill folder with only the
// files setup installed; the harness loads everything it finds in those folders.
const backupsDirName = "backups"

// backupStore finds the copies setup keeps in the state dir. A zero store
// (bases unknown) knows nothing, so the copy stays next to the file.
type backupStore struct{ home, state string }

type storedBackup struct {
	file, stamp string
	marker      bool
}

// storeOf reads the home and state dir out of Options.LinkBases.
func storeOf(bases []string) backupStore {
	if len(bases) < 2 {
		return backupStore{}
	}
	return backupStore{home: bases[0], state: bases[1]}
}

func (o Options) store() backupStore { return storeOf(o.LinkBases()) }

// rel is the path below the home dir, when path may be kept in the store: it
// must be under home and outside the state dir (already a folder only setup uses).
func (s backupStore) rel(path string) (string, bool) {
	if s.home == "" || !withinHome(s.home, path) || withinHome(s.state, path) {
		return "", false
	}
	rel, err := filepath.Rel(s.home, path)
	return rel, err == nil
}

// location is where Write keeps the marker or backup of path; ok is false when
// it stays next to the file.
func (s backupStore) location(path, stamp string, marker bool) (string, bool) {
	rel, ok := s.rel(path)
	if !ok {
		return "", false
	}
	dest := filepath.Join(s.state, backupsDirName, stamp, rel)
	if marker {
		dest += ".new"
	}
	return dest, true
}

// stored lists what the store keeps for path, oldest first.
func (s backupStore) stored(path string) []storedBackup {
	rel, ok := s.rel(path)
	if !ok {
		return nil
	}
	root := filepath.Join(s.state, backupsDirName)
	stamps, _ := os.ReadDir(root)
	var out []storedBackup
	for _, st := range stamps {
		base := filepath.Join(root, st.Name(), rel)
		if _, err := os.Lstat(base); err == nil {
			out = append(out, storedBackup{base, st.Name(), false})
		}
		if _, err := os.Lstat(base + ".new"); err == nil {
			out = append(out, storedBackup{base + ".new", st.Name(), true})
		}
	}
	return out
}

// drop removes one stored copy and the stamp folders it leaves empty.
func (s backupStore) drop(c storedBackup) {
	os.Remove(c.file)
	root := filepath.Join(s.state, backupsDirName)
	for dir := filepath.Dir(c.file); dir != root && withinHome(root, dir); dir = filepath.Dir(dir) {
		if os.Remove(dir) != nil { // not empty: stop
			return
		}
	}
}

// LatestBackup returns the newest backup or marker for path, wherever it is
// kept: in the state dir (current layout) or next to the file (older setups).
func (o Options) LatestBackup(path string) (backup string, marker bool) {
	best, marker := LatestBackup(path)
	bestStamp := ""
	if best != "" {
		bestStamp = strings.TrimSuffix(strings.TrimPrefix(filepath.Base(best), filepath.Base(path)+BackupInfix), ".new")
	}
	for _, c := range o.store().stored(path) {
		if best == "" || c.stamp > bestStamp {
			best, bestStamp, marker = c.file, c.stamp, c.marker
		}
	}
	return best, marker
}

// removeMarkers deletes the spent "did not exist" markers of path in both
// layouts, and the stamp folders they leave empty.
func (o Options) removeMarkers(path string) {
	stale, _ := filepath.Glob(path + BackupInfix + "*.new")
	for _, m := range stale {
		os.Remove(m)
	}
	s := o.store()
	for _, c := range s.stored(path) {
		if c.marker {
			s.drop(c)
		}
	}
}

// pruneOlderBackups removes the full backups of path older than the one just
// written (keep, made at stamp), in both layouts. Undo restores only the
// newest one, so the rest pile up on every setup (a mod folder once held 171
// of them, all inside a folder the harness watches and reloads). The empty
// "did not exist" markers stay: they record that setup created the file.
func pruneOlderBackups(bases []string, path, keep, stamp string) {
	matches, _ := filepath.Glob(path + BackupInfix + "*")
	for _, m := range matches {
		if m == keep || strings.HasSuffix(m, ".new") || m > keep {
			continue
		}
		os.Remove(m)
	}
	s := storeOf(bases)
	for _, c := range s.stored(path) {
		if !c.marker && c.file != keep && c.stamp < stamp {
			s.drop(c)
		}
	}
}
