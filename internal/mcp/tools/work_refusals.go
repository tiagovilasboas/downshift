// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package tools

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Refusal codes delegate reports for a task or a call it did not start.
const (
	// RefusalCap: the session already started its maximum number of workers.
	RefusalCap = "cap"
	// RefusalDir: the dir resolves outside the project directory.
	RefusalDir = "dir"
)

const dirFix = "fix: pass a dir inside the project directory the server was started in, or omit dir"

// errDirOutside is the refusal for a dir that escapes the server root.
var errDirOutside = errors.New("dir is outside the project directory (" + RefusalDir + "); " + dirFix)

// insideRoot reports whether dir, with symlinks resolved, lies inside root.
// Read and write jobs share it. A path that does not exist yet is resolved
// through its deepest existing ancestor, so a symlinked parent still counts.
//
// A ".." element is refused before any cleaning: filepath.Abs cleans the path
// as text, so "<root>/link/.." would read as the root while the kernel follows
// the link first. It returns the resolved directory, which is the one the
// worker runs in, so the checked path and the used path are the same.
func insideRoot(root, dir string) (string, error) {
	if hasDotDot(dir) {
		return "", errDirOutside
	}
	r, err := resolveLoose(root)
	if err != nil {
		return "", fmt.Errorf("project directory cannot be resolved (%s); %s", RefusalDir, dirFix)
	}
	d, err := resolveLoose(dir)
	if err != nil {
		return "", fmt.Errorf("dir cannot be resolved (%s); %s", RefusalDir, dirFix)
	}
	rel, err := filepath.Rel(r, d)
	if err != nil || filepath.IsAbs(rel) || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", errDirOutside
	}
	return d, nil
}

// hasDotDot reports whether p has a ".." element, with either separator.
func hasDotDot(p string) bool {
	for _, part := range strings.FieldsFunc(p, func(r rune) bool { return r == '/' || r == '\\' }) {
		if part == ".." {
			return true
		}
	}
	return false
}

// resolveLoose cleans p and resolves symlinks in the part that exists.
func resolveLoose(p string) (string, error) {
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	rest := ""
	for cur := abs; ; {
		if real, err := filepath.EvalSymlinks(cur); err == nil {
			return filepath.Join(real, rest), nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return abs, nil
		}
		rest = filepath.Join(filepath.Base(cur), rest)
		cur = parent
	}
}
