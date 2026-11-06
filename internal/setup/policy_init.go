// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package setup

import (
	"errors"
	"io/fs"
	"os"

	"github.com/tiagovilasboas/downshift/internal/routing/policy"
)

// PolicyInitDocument is the file `downshift policy init` creates. It holds the
// version and nothing else: a file without a routing key has no routing policy
// and sets no tier floor, so the new file changes no routing until the user
// adds a section.
const PolicyInitDocument = "{\n  \"version\": 1\n}\n"

// policyInitTarget labels the plan in the output. The file belongs to the
// project, not to a harness.
var policyInitTarget = Target{Name: "policy"}

// PlanPolicyInit plans the project policy file under o.ProjectDir. It never
// writes. ok is false when no project directory is set, which is how setup
// skips the step in tests and in the paths that must not touch a project.
func PlanPolicyInit(o Options) (plan Plan, ok bool) {
	if o.ProjectDir == "" {
		return Plan{}, false
	}
	path := policy.ProjectPath(o.ProjectDir)
	plan = Plan{Target: policyInitTarget, Path: path, After: []byte(PolicyInitDocument)}
	_, err := os.Lstat(path)
	switch {
	case err == nil:
		plan.Skip = Display(o.Home, path) + " already exists, so it is left untouched"
	case errors.Is(err, fs.ErrNotExist):
	default:
		plan.Err = err
	}
	return plan, true
}
