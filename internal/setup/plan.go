// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package setup

import (
	"os"
	"path/filepath"
	"strings"
)

// Plan is the outcome of planning one target.
type Plan struct {
	Target  Target
	Path    string
	Before  []byte // nil when the file does not exist
	Existed bool
	After   []byte
	Notes   []string
	Err     error
	// Mode is the permission of a file setup creates or rewrites; 0 keeps the
	// default (the existing file's mode, or 0600 for a new one).
	Mode os.FileMode
	// Bases are the roots setup checked the path against for links
	// (o.LinkBases); Write repeats the check when they are set.
	Bases []string
	// Loaded marks a file inside a folder the harness loads (a plugin, skill or
	// mod folder): its backup is kept in the state dir, not beside the file.
	Loaded bool
	// Skip is why nothing is planned for this target (not an error).
	Skip string
	// Companions are files the config points at (a generated wrapper). They are
	// written first, and the config is not written if one of them fails.
	Companions []Plan
}

func (p Plan) Changed() bool {
	return p.Err == nil && p.Skip == "" && (!p.Existed || string(p.Before) != string(p.After))
}

// Display shortens a path under HOME to ~/....
func Display(home, p string) string {
	if rel, err := filepath.Rel(home, p); err == nil && !strings.HasPrefix(rel, "..") {
		return "~/" + filepath.ToSlash(rel)
	}
	return p
}
