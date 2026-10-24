// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package harness2export

import "fmt"

// Judge checks the lab invariant. A harness mutates only when its matrix
// status is Completed and the binary has a hook. Observation and config-only
// rows stay ineligible. The public eight-prompt file is not an input here.
func Judge(rows []Row) error {
	if len(rows) == 0 {
		return fmt.Errorf("lab matrix is empty")
	}
	seen := map[string]bool{}
	mutators := 0
	for _, row := range rows {
		if seen[row.ID] {
			return fmt.Errorf("duplicate harness %q", row.ID)
		}
		seen[row.ID] = true
		want := RoleOf(row.Maturity)
		if row.Role != want {
			return fmt.Errorf("%s role %q, want %q for maturity %q", row.ID, row.Role, want, row.Maturity)
		}
		eligible := want == RoleMutate && row.HasHook
		if row.RecursiveHarness2Eligible != eligible {
			return fmt.Errorf("%s eligible=%v, want %v", row.ID, row.RecursiveHarness2Eligible, eligible)
		}
		if eligible {
			mutators++
		}
	}
	if mutators == 0 {
		return fmt.Errorf("lab has no Completed harness with a hook to mutate")
	}
	return nil
}
