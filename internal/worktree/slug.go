// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package worktree implements the isolated worktree rules of the plan runner
// (stage 4): the slug that names a worktree directory and a branch segment
// (R54), and the containment check that keeps every write under the worktree
// root (R56).
package worktree

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// ErrInvalidSlug is wrapped by every error that ValidateSlug returns.
var ErrInvalidSlug = errors.New("invalid slug")

// slugPattern is the R54 grammar: a lowercase letter or digit, then up to 63
// lowercase letters, digits, dots, underscores or hyphens. It also refuses
// spaces, path separators, control bytes, "@{" and a leading hyphen.
var slugPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}$`)

// ValidateSlug accepts a worker id only when it is safe as a directory name and
// as a branch segment on macOS, Linux and Windows. The R54 grammar runs first;
// then the rules the grammar cannot express: no ".." anywhere, no ".lock"
// ending, no trailing dot, and no Windows device name as the part before the
// first dot. Every error wraps ErrInvalidSlug and names the rule it broke.
func ValidateSlug(slug string) error {
	if !slugPattern.MatchString(slug) {
		return refuseSlug(slug, "does not match ^[a-z0-9][a-z0-9._-]{0,63}$")
	}
	switch {
	case strings.Contains(slug, ".."):
		return refuseSlug(slug, `contains ".."`)
	case strings.HasSuffix(slug, ".lock"):
		return refuseSlug(slug, `ends with ".lock"`)
	case strings.HasSuffix(slug, "."):
		return refuseSlug(slug, "ends with a dot")
	case isWindowsDevice(slug):
		return refuseSlug(slug, "is a Windows device name")
	}
	return nil
}

// isWindowsDevice reports whether the part of slug before its first dot is a
// device name that Windows reserves: CON, PRN, AUX, NUL, COM1 to COM9 and LPT1
// to LPT9. The name stays reserved with an extension, so "nul.tar.gz" is NUL.
// It expects a lowercase slug, which ValidateSlug guarantees before calling it.
func isWindowsDevice(slug string) bool {
	stem := strings.SplitN(slug, ".", 2)[0]
	switch stem {
	case "con", "prn", "aux", "nul":
		return true
	}
	if len(stem) != 4 || stem[3] < '1' || stem[3] > '9' {
		return false
	}
	return strings.HasPrefix(stem, "com") || strings.HasPrefix(stem, "lpt")
}

func refuseSlug(slug, rule string) error {
	return fmt.Errorf("%w: %q %s", ErrInvalidSlug, slug, rule)
}
