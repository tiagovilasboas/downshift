// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// compactSteering is the versioned compact steering every harness with an
// instructions file receives. It is the body only; a harness that needs
// frontmatter (KiroCrew) declares it on its setupSteering. The source of truth
// is the embedded file, never a hand-written copy on one machine. The family
// list inside it is checked against execwrap.FamilyOf by setup_steering_test.go.
//
//go:embed assets/compact-steering.md
var compactSteering string

// Markers around the block setup owns inside a shared instructions file.
const (
	steeringBlockBegin = "<!-- downshift:compact:begin -->"
	steeringBlockEnd   = "<!-- downshift:compact:end -->"
)

// setupSteering is an optional steering file a target installs. Path is
// relative to HOME. Two shapes:
//
//   - a dedicated file (KiroCrew's workspace steering): setup writes the whole
//     file, Frontmatter first, and rewrites it only when it differs;
//   - a Managed file (Codex, OpenCode, Antigravity global instructions): the
//     user owns the file, so setup keeps everything outside the marked block
//     and only inserts or replaces that block.
//
// Both are idempotent and `--undo`-able through the same backup path as hook
// files.
type setupSteering struct {
	Path        string // relative to HOME
	Frontmatter string // written before Content for a dedicated file
	Content     string
	Managed     bool
}

// planSteering plans the steering write for a target that declares one,
// reusing setupPlan so the diff, backup and undo behave like a hook file.
// A target with no Steering returns ok=false.
func planSteering(o setupOptions, t setupTarget) (p setupPlan, ok bool) {
	if t.Steering == nil {
		return setupPlan{}, false
	}
	path := filepath.Join(o.Home, filepath.FromSlash(t.Steering.Path))
	p = setupPlan{target: t, path: path}
	if b, err := os.ReadFile(path); err == nil {
		p.before = b
		p.existed = true
	} else if !os.IsNotExist(err) {
		p.err = err
		return p, true
	}
	if t.Steering.Managed {
		p.after = mergeSteeringBlock(p.before, t.Steering.Content)
		return p, true
	}
	// A dedicated file is setup's own. Unmarked content is a file a person wrote,
	// except the exact text an earlier setup wrote before the marker existed.
	legacy := []byte(t.Steering.Frontmatter + t.Steering.Content)
	if p.existed && !setupOwns(p.before) && !bytes.Equal(p.before, legacy) {
		p.err = fmt.Errorf("%s was not written by downshift setup; move it aside and run setup again", path)
		return p, true
	}
	p.after = []byte(t.Steering.Frontmatter + setupOwnedNote + t.Steering.Content)
	return p, true
}

// mergeSteeringBlock returns before with the managed block set to body. A
// block already present is replaced in place, so a second run is a no-op; a
// file without the block gets the block appended after a blank line. Text
// outside the markers is never touched.
func mergeSteeringBlock(before []byte, body string) []byte {
	block := steeringBlockBegin + "\n" + strings.TrimRight(body, "\n") + "\n" + steeringBlockEnd
	text := string(before)
	start := strings.Index(text, steeringBlockBegin)
	end := strings.Index(text, steeringBlockEnd)
	if start >= 0 && end > start {
		rest := strings.TrimPrefix(text[end+len(steeringBlockEnd):], "\n")
		out := text[:start] + block + "\n"
		if rest != "" {
			out += rest
		}
		return []byte(out)
	}
	if text == "" {
		return []byte(block + "\n")
	}
	if !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	return []byte(text + "\n" + block + "\n")
}
