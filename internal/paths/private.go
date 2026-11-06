// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package paths

import (
	"errors"
	"os"
)

// ErrUnsafeDir is returned for a directory that is a symlink, not a directory,
// or owned by another user.
var ErrUnsafeDir = errors.New("paths: directory is a symlink or not owned by the caller")

// PrivateDir creates dir with DirPerm and refuses it when it is a symlink, not
// a directory, or (where ownership is known) owned by someone else. A dir the
// caller owns but left wider is tightened. Use it for a directory that holds
// private data, so a planted directory or link is never written through.
func PrivateDir(dir string) error {
	if err := os.MkdirAll(dir, DirPerm); err != nil {
		return err
	}
	info, err := os.Lstat(dir)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() || (ownerKnown && !ownedByCaller(info)) {
		return ErrUnsafeDir
	}
	tighten(dir, DirPerm)
	return nil
}

// RegularFile reports whether p exists as a regular file, not following a
// symlink. It returns os.ErrNotExist when p is missing and
// sibling error when p is a link or a special file.
func RegularFile(p string) (os.FileInfo, error) {
	info, err := os.Lstat(p)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, ErrNotRegular
	}
	return info, nil
}

// ErrNotRegular is returned for a path that is a symlink or a special file.
var ErrNotRegular = errors.New("paths: not a regular file")
