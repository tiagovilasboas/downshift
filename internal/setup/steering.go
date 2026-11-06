// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package setup

import (
	_ "embed"
	"strings"
)

// CompactSteering is the versioned compact steering every harness with an
// instructions file receives. It is the body only; a harness that needs
// frontmatter (KiroCrew) declares it on its Steering. The source of truth
// is the embedded file, never a hand-written copy on one machine. The family
// list inside it is checked against execwrap.FamilyOf by steering_plan_test.go.
//
//go:embed assets/compact-steering.md
var CompactSteering string

// AgentTriggers tells the model that "downshift full power" and the other
// trigger phrases mean dispatching subagents now. It rides in every harness's
// always-loaded instructions: a skill loads only when the model chooses to
// open it, so a trigger kept only in a skill can be missed.
//
//go:embed assets/agent-triggers.md
var AgentTriggers string

// HarnessSteering is what a harness without a shell-rewrite hook receives:
// the compact steering, then the agent triggers.
var HarnessSteering = CompactSteering + "\n" + AgentTriggers

// Markers around the block setup owns inside a shared instructions file.
const (
	SteeringBlockBegin = "<!-- downshift:compact:begin -->"
	SteeringBlockEnd   = "<!-- downshift:compact:end -->"
	// The agent-trigger block, for a harness that needs no compact steering
	// (Claude Code compacts through its shell-rewrite hook).
	AgentsBlockBegin = "<!-- downshift:agents:begin -->"
	AgentsBlockEnd   = "<!-- downshift:agents:end -->"
)

// Steering is an optional steering file a target installs. Path is
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
type Steering struct {
	Path        string // relative to HOME
	Frontmatter string // written before Content for a dedicated file
	Content     string
	Managed     bool
	// Begin and End mark the managed block; empty means the compact markers.
	Begin, End string
}

// markers returns the managed block's marker lines.
func (s Steering) Markers() (string, string) {
	if s.Begin != "" && s.End != "" {
		return s.Begin, s.End
	}
	return SteeringBlockBegin, SteeringBlockEnd
}

// MergeMarkedBlock is mergeSteeringBlock for any pair of marker lines, so a
// file format with another comment syntax (the Codex TOML) reuses it.
func MergeMarkedBlock(before []byte, begin, end, body string) []byte {
	block := begin + "\n" + strings.TrimRight(body, "\n") + "\n" + end
	text := string(before)
	start := strings.Index(text, begin)
	stop := strings.Index(text, end)
	if start >= 0 && stop > start {
		rest := strings.TrimPrefix(text[stop+len(end):], "\n")
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
