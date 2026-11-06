// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package configfile

import (
	"os"
	"path/filepath"
)

// WriteAtomic replaces path with data without ever exposing a partial or
// wider-than-mode file. setup.Write and setup.Undo refuse a path with a linked
// component before calling it, so it never writes through a link. The temporary file is unique, lives next to the target (same
// filesystem for the rename), is synced before the rename, and the directory
// is synced after it so a crash leaves the old or the new content, never an
// empty file.
func WriteAtomic(path string, data []byte, mode os.FileMode) error {
	target := path
	dir := filepath.Dir(target)
	tmp, err := os.CreateTemp(dir, filepath.Base(target)+".downshift-tmp-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	fail := func(err error) error {
		tmp.Close()
		os.Remove(name)
		return err
	}
	if err := tmp.Chmod(mode); err != nil {
		return fail(err)
	}
	if _, err := tmp.Write(data); err != nil {
		return fail(err)
	}
	if err := tmp.Sync(); err != nil {
		return fail(err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(name)
		return err
	}
	if err := os.Rename(name, target); err != nil {
		os.Remove(name)
		return err
	}
	if d, err := os.Open(dir); err == nil {
		_ = d.Sync()
		d.Close()
	}
	return nil
}

// RemoveEmptyDirs removes dir and up to depth-1 of its parents while they are
// empty; os.Remove refuses a directory that still holds anything. It stops at
// a link or anything that is not a plain directory: os.Remove would unlink a
// symlink even when its target holds files.
func RemoveEmptyDirs(dir string, depth int) {
	for i := 0; i < depth; i++ {
		if st, err := os.Lstat(dir); err != nil || !st.IsDir() || st.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 {
			return
		}
		if os.Remove(dir) != nil {
			return
		}
		dir = filepath.Dir(dir)
	}
}
