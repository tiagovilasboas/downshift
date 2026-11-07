// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"github.com/tiagovilasboas/downshift/internal/ccr"
)

// ccrKeep returns the port that stores the raw output of every compacted run
// for `downshift retrieve`. CCR is part of the compactor, not an option: there
// is no switch, file key or environment variable that turns it off (only
// "exec": false / DOWNSHIFT_EXEC=0, which turns compaction itself off, so
// nothing is cut and nothing needs keeping). It is nil only when the store
// cannot be opened, and a store error leaves the compacted output as it is,
// without an id.
func ccrKeep() func(raw []byte) (string, bool) {
	store, err := ccr.Default()
	if err != nil {
		return nil
	}
	return func(raw []byte) (string, bool) {
		id, err := store.Put(raw)
		return id, err == nil
	}
}
