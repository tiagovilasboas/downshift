// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package claudecode

import (
	"os"

	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/policyfile"
)

// applyPolicyFloor raises the decision to the project's tier floor, if a valid
// .downshift/policy.json applies. Any problem (no file, invalid file, no cwd)
// leaves the decision untouched, so the hook fails open. cwd is the hook
// payload's working directory; when absent the process directory is used.
func applyPolicyFloor(d *core.Decision, cwd, currentModel string, res core.Resolver) {
	if cwd == "" {
		cwd, _ = os.Getwd()
	}
	p, _, err := policyfile.LoadNearest(cwd)
	if err != nil || p == nil {
		return
	}
	if tier, ok := p.Floor(d.Complexity.String(), d.Risk); ok {
		d.RaiseTier(tier, currentModel, res)
	}
}
