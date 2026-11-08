// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"github.com/tiagovilasboas/downshift-ai/internal/core"
	"github.com/tiagovilasboas/downshift-ai/internal/mcp/tools"
	"github.com/tiagovilasboas/downshift-ai/internal/routing/policy"
)

// mcpParallel resolves the squad fan-out cap of one harness: the user's
// override, the harness's declared cap, the policy files (project under root,
// then the user's), then the default. It reads the files on every call, so an
// edit applies to the next one.
func mcpParallel(root string, getenv func(string) string) func(string) tools.ParallelCap {
	return func(harness string) tools.ParallelCap {
		user, notes := policy.EnvParallel(getenv)
		loc := policy.Locations{}
		if root != "" {
			loc.Project = policy.ProjectPath(root)
		}
		if u, err := policy.UserPath(); err == nil {
			loc.User = u
		}
		fromFiles, fileNotes := policy.FilesParallel(loc)
		notes = append(notes, fileNotes...)
		harnessCap := 0
		for _, n := range core.AllHarnessCaps() {
			if n.Harness == harness {
				harnessCap = n.Caps.MaxParallel
			}
		}
		p := policy.ResolveParallel(user, harnessCap, fromFiles, notes)
		return tools.ParallelCap{Max: p.Max, WhenStrained: p.WhenStrained, Source: p.Source, Notes: p.Notes}
	}
}
