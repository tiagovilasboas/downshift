// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package accounting

import (
	"github.com/tiagovilasboas/downshift-ai/internal/execwrap"
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

// ReadSkipEvents reads the skip log of the state directory: the Bash calls the
// hook declined to wrap. A missing log is an empty slice, not an error.
func ReadSkipEvents() ([]execwrap.SkipEvent, error) {
	path, err := execwrap.SkipsPath()
	if err != nil {
		return nil, err
	}
	return execwrap.ReadSkips(path)
}
