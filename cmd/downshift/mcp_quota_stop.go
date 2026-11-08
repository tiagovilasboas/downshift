// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import "github.com/tiagovilasboas/downshift-ai/internal/routing/policy"

// mcpQuotaStop resolves the delegate stop line: the user's override, the policy
// files (project under root, then the user's), then the default. It reads the
// files on every call, so an edit applies to the next one.
func mcpQuotaStop(root string, getenv func(string) string) func() float64 {
	return func() float64 {
		user, _ := policy.EnvQuotaStop(getenv)
		loc := policy.Locations{}
		if root != "" {
			loc.Project = policy.ProjectPath(root)
		}
		if u, err := policy.UserPath(); err == nil {
			loc.User = u
		}
		file, _ := policy.FilesQuotaStop(loc)
		return policy.ResolveQuotaStop(user, file)
	}
}
