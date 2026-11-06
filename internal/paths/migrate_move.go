// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package paths

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// renameFn is os.Rename; tests replace it to force the copy path.
var renameFn = os.Rename

// legacyEntries lists the legacy directory's state entries (not the note).
func legacyEntries(dir string) ([]string, error) {
	ents, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range ents {
		if e.Name() != MovedNote {
			names = append(names, e.Name())
		}
	}
	return names, nil
}

func dirEntryCount(dir string) (int, error) {
	ents, err := os.ReadDir(dir)
	return len(ents), err
}

// moveEntries puts every legacy entry into the target before it deletes any
// legacy copy. An entry is renamed when possible and copied otherwise (for
// example across devices). On any failure the target is rolled back (renamed
// entries return, copies are removed) and the legacy dir is left whole, so a
// half-written target never shadows it. A copy that cannot be removed from the
// legacy dir afterwards is a warning: the data is safe in the target.
func moveEntries(m Migration, names []string) (warnings []string, err error) {
	if err := os.MkdirAll(m.Target, 0o700); err != nil {
		return nil, err
	}
	var copied []string
	var renamed []string
	for _, n := range names {
		src, dst := filepath.Join(m.Legacy, n), filepath.Join(m.Target, n)
		if renameFn(src, dst) == nil {
			renamed = append(renamed, n)
			continue
		}
		copied = append(copied, n)
		if err := copyTree(src, dst); err != nil {
			rollback(m, renamed, copied)
			return nil, fmt.Errorf("copy %s: %w", n, err)
		}
	}
	for _, n := range copied {
		if err := os.RemoveAll(filepath.Join(m.Legacy, n)); err != nil {
			warnings = append(warnings, fmt.Sprintf("copied, could not remove legacy %s: %v", filepath.Join(m.Legacy, n), err))
		}
	}
	return warnings, nil
}

// rollback undoes a partial move: renamed entries go back, copies are removed.
func rollback(m Migration, renamed, copied []string) {
	for _, n := range renamed {
		_ = renameFn(filepath.Join(m.Target, n), filepath.Join(m.Legacy, n))
	}
	for _, n := range copied {
		_ = os.RemoveAll(filepath.Join(m.Target, n))
	}
}

// copyTree copies a file, symlink or directory tree. A symlink is recreated,
// never followed; a failure to do so is an error, not a content copy.
func copyTree(src, dst string) error {
	info, err := os.Lstat(src)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		link, err := os.Readlink(src)
		if err != nil {
			return err
		}
		return os.Symlink(link, dst)
	}
	if !info.IsDir() {
		return copyFile(src, dst, info.Mode().Perm())
	}
	if err := os.MkdirAll(dst, 0o700); err != nil {
		return err
	}
	ents, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, e := range ents {
		if err := copyTree(filepath.Join(src, e.Name()), filepath.Join(dst, e.Name())); err != nil {
			return err
		}
	}
	return nil
}

func copyFile(src, dst string, perm os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// collisions lists the legacy entry names that also exist in the target.
func collisions(target string, names []string) []string {
	var out []string
	for _, n := range names {
		if _, err := os.Lstat(filepath.Join(target, n)); err == nil {
			out = append(out, n)
		}
	}
	return out
}
