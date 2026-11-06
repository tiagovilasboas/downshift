// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package operatorexport

import (
	"sort"

	"github.com/tiagovilasboas/downshift/internal/core"
)

// knownHarnessNames is the set of adapter names that may appear as a harness
// value in the export, read from the one capability table so a new adapter
// needs no edit here. Sorted for a stable allowlist and error text.
func knownHarnessNames() []string {
	var names []string
	for _, n := range core.AllHarnessCaps() {
		names = append(names, n.Harness)
	}
	sort.Strings(names)
	return names
}
