// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package setup

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tiagovilasboas/downshift-ai/internal/setup/configfile"
)

// planSteering plans the steering write for a target that declares one,
// reusing Plan so the diff, backup and undo behave like a hook file.
// A target with no Steering returns ok=false.
func planSteering(o Options, t Target) (p Plan, ok bool) {
	if t.Steering == nil {
		return Plan{}, false
	}
	path := filepath.Join(o.Home, filepath.FromSlash(t.Steering.Path))
	p = Plan{Target: t, Path: path}
	if b, err := os.ReadFile(path); err == nil {
		p.Before = b
		p.Existed = true
	} else if !os.IsNotExist(err) {
		p.Err = err
		return p, true
	}
	if t.Steering.Managed {
		begin, end := t.Steering.Markers()
		p.After = MergeMarkedBlock(p.Before, begin, end, steeringFor(t))
		return p, true
	}
	// A dedicated file is setup's own. Unmarked content is a file a person wrote,
	// except the exact text an earlier setup wrote before the marker existed.
	// The compact-only text is the legacy shape from before the agent triggers.
	legacy := []byte(t.Steering.Frontmatter + t.Steering.Content)
	legacyCompact := []byte(t.Steering.Frontmatter + CompactSteering)
	named := []byte(t.Steering.Frontmatter + steeringFor(t))
	if p.Existed && !configfile.Owns(p.Before) && !bytes.Equal(p.Before, legacy) && !bytes.Equal(p.Before, legacyCompact) && !bytes.Equal(p.Before, named) {
		p.Err = fmt.Errorf("%s was not written by downshift setup; move it aside and run setup again", path)
		return p, true
	}
	p.After = []byte(t.Steering.Frontmatter + configfile.OwnedNote + steeringFor(t))
	return p, true
}

// mergeSteeringBlock returns before with the managed block set to body. A
// block already present is replaced in place, so a second run is a no-op; a
// file without the block gets the block appended after a blank line. Text
// outside the markers is never touched.
func mergeSteeringBlock(before []byte, body string) []byte {
	return MergeMarkedBlock(before, SteeringBlockBegin, SteeringBlockEnd, body)
}

// compactCommand is how the steering tells an agent to run the compactor.
const compactCommand = "downshift compactor -- "

// steeringFor is the target's steering with the compactor command naming the
// harness (`downshift compactor --harness=<name> -- `), so a run the agent
// starts itself is counted for its harness even when the harness exports no
// session variable to the shell.
func steeringFor(t Target) string {
	return strings.ReplaceAll(t.Steering.Content, compactCommand, "downshift compactor --harness="+t.Name+" -- ")
}
