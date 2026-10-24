// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package accounting

import (
	"github.com/tiagovilasboas/downshift/internal/execwrap"
)

// ReadExecEvents reads the exec log of the state directory. A missing log is
// an empty slice, not an error.
func ReadExecEvents() ([]execwrap.Event, error) {
	path, err := execwrap.EventsPath()
	if err != nil {
		return nil, err
	}
	return execwrap.ReadEvents(path)
}
