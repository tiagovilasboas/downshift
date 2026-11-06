// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package paths

import (
	"os"
	"path/filepath"
	"strings"
)

const (
	// DirPerm is the mode of the state directory; FilePerm of the files in it.
	DirPerm  os.FileMode = 0o700
	FilePerm os.FileMode = 0o600
)

// EnsureStateDir creates the state directory with 0700. When the directory is
// the default one (not a path the user chose with DOWNSHIFT_STATE_DIR, which
// may be shared) and an older release left it wider, it is tightened, provided
// the caller owns it.
func EnsureStateDir() (string, error) {
	dir, source, err := Resolve()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, DirPerm); err != nil {
		return "", err
	}
	if source != SourceEnv {
		tighten(dir, DirPerm)
	}
	return dir, nil
}

// ownFile names the files Downshift keeps in the default state directory.
// Anything else is left alone: only files the product created are touched.
func ownFile(name string) bool {
	if strings.Contains(name, ".downshift-bak-") {
		return false
	}
	switch filepath.Ext(name) {
	case ".json", ".jsonl", ".lock", ".failed", ".log", ".tmp":
		return true
	}
	return false
}

// Harden is EnsureStateDir plus a one-time tightening of the product's own
// files (0600) that older releases wrote with the default umask. Best effort:
// it never fails the caller for a file it cannot change.
func Harden() error {
	dir, err := EnsureStateDir()
	if err != nil {
		return err
	}
	if _, source, _ := Resolve(); source == SourceEnv {
		return nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	for _, e := range entries {
		if e.Type().IsRegular() && ownFile(e.Name()) {
			tighten(filepath.Join(dir, e.Name()), FilePerm)
		}
	}
	return nil
}

// tighten removes group and other access when the caller owns the path and it
// is not a symlink. It never widens a mode.
func tighten(path string, perm os.FileMode) {
	info, err := os.Lstat(path)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !ownedByCaller(info) {
		return
	}
	if info.Mode().Perm()&^perm == 0 {
		return
	}
	_ = os.Chmod(path, info.Mode().Perm()&perm)
}
