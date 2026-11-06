// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import "github.com/tiagovilasboas/downshift/internal/routing/policy"

// mcpBudgetTokens resolves the orchestration token cap: the user's override,
// the policy files (project under root, then the user's), then none (0). It
// reads the files on every call, so an edit applies to the next one.
func mcpBudgetTokens(root string, getenv func(string) string) func() int64 {
	return func() int64 {
		user, _ := policy.EnvBudgetTokens(getenv)
		loc := policy.Locations{}
		if root != "" {
			loc.Project = policy.ProjectPath(root)
		}
		if u, err := policy.UserPath(); err == nil {
			loc.User = u
		}
		file, _ := policy.FilesBudgetTokens(loc)
		return policy.ResolveBudgetTokens(user, file)
	}
}
