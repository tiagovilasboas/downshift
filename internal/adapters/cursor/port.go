// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package cursor

import "github.com/tiagovilasboas/downshift/internal/hookport"

// Port registers only the child-end parser (`downshift end cursor`). Honor and
// usage stay nil: this harness's hooks do not send those fields to the legacy
// runners, and absence is not inference.
func Port() hookport.Port {
	return hookport.Port{ID: harnessID, ParseChildEnd: ParseChildEnd}
}
