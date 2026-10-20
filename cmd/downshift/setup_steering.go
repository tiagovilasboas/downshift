// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	_ "embed"
	"os"
	"path/filepath"
)

// kirocrewCompactSteering is the versioned steering asset `setup` installs into
// the KiroCrew workspace so every agent turn is told to wrap test/build/status
// commands in `downshift compact`. KiroCrew has no shell-rewrite hook channel,
// so this guia is the only way compaction reaches it; the source of truth is
// the embedded file, never a hand-written copy on one machine.
//
//go:embed assets/kirocrew-compact-steering.md
var kirocrewCompactSteering string

// setupSteering is an optional steering file a target installs. Path is
// relative to HOME. Unlike a hook File it is a whole-file asset: setup writes
// Content when the file is absent or differs, and leaves it untouched when it
// already matches, so it is idempotent and `--undo`-able through the same
// backup path as hook files.
type setupSteering struct {
	Path    string // relative to HOME
	Content string
}

// planSteering plans the steering write for a target that declares one,
// reusing setupPlan so the diff, backup and undo behave like a hook file.
// A target with no Steering returns ok=false.
func planSteering(o setupOptions, t setupTarget) (p setupPlan, ok bool) {
	if t.Steering == nil {
		return setupPlan{}, false
	}
	path := filepath.Join(o.Home, filepath.FromSlash(t.Steering.Path))
	p = setupPlan{target: t, path: path, after: []byte(t.Steering.Content)}
	if b, err := os.ReadFile(path); err == nil {
		p.before = b
		p.existed = true
	} else if !os.IsNotExist(err) {
		p.err = err
	}
	return p, true
}
