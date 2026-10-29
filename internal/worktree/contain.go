// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package worktree

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// Contained reports whether candidate is root or lies under it (R56). Both
// paths are resolved through their symlinks before the comparison, and the
// comparison uses filepath.Rel, never a string prefix. The root must exist.
// The candidate may not exist yet: its nearest existing ancestor is resolved
// and the missing names are appended. A dangling symlink is an error, because
// nothing says where it points.
//
// Both paths must be absolute and must not contain a ".." element. Lexical
// cleaning would fold "link/../x" before the symlink "link" is resolved, which
// can name a different directory than the kernel opens. Drive letters compare
// case-insensitively, as filepath.Rel does on Windows.
func Contained(root, candidate string) (bool, error) {
	if err := checkContainPath(root); err != nil {
		return false, fmt.Errorf("root: %w", err)
	}
	if err := checkContainPath(candidate); err != nil {
		return false, fmt.Errorf("candidate: %w", err)
	}
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return false, fmt.Errorf("resolve root: %w", err)
	}
	realCandidate, err := resolveExisting(candidate)
	if err != nil {
		return false, fmt.Errorf("resolve candidate: %w", err)
	}
	rel, err := filepath.Rel(realRoot, realCandidate)
	if err != nil {
		// The paths sit on different volumes, so nothing is relative to root.
		return false, nil
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return false, nil
	}
	return true, nil
}

// checkContainPath refuses the inputs the containment rule does not define.
func checkContainPath(p string) error {
	switch {
	case p == "":
		return errors.New("empty path")
	case !filepath.IsAbs(p):
		return fmt.Errorf("path %q is not absolute", p)
	case hasDotDotElement(p):
		return fmt.Errorf("path %q has a \"..\" element", p)
	}
	return nil
}

// hasDotDotElement reports whether p has a ".." element, split on both slash
// kinds so that the check is the same on every OS.
func hasDotDotElement(p string) bool {
	elems := strings.FieldsFunc(p, func(r rune) bool {
		return r == '/' || r == filepath.Separator
	})
	return slices.Contains(elems, "..")
}

// resolveExisting returns the real path of p. When p does not exist, it
// resolves the nearest existing ancestor and appends the missing names. A
// dangling symlink is an error: its target is unknown, so it cannot be judged.
func resolveExisting(p string) (string, error) {
	var missing []string
	cur := filepath.Clean(p)
	for {
		resolved, err := filepath.EvalSymlinks(cur)
		if err == nil {
			return filepath.Join(append([]string{resolved}, missing...)...), nil
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return "", err
		}
		if _, lerr := os.Lstat(cur); lerr == nil {
			return "", fmt.Errorf("dangling symlink %s", cur)
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return "", err
		}
		missing = append([]string{filepath.Base(cur)}, missing...)
		cur = parent
	}
}
