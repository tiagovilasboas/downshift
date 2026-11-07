// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"

	"github.com/tiagovilasboas/downshift/internal/ccr"
	"github.com/tiagovilasboas/downshift/internal/contextopt"
)

// ccrKeep returns the ports that store raw output for `downshift retrieve`.
// all is set when CCR is fully on, which is the default ("ccr": false or
// DOWNSHIFT_CCR=0 turns it off): it keeps every compacted run. failed is what
// remains with full CCR off: it keeps only a run that exited non-zero, unless
// "ccr_failures": false or DOWNSHIFT_CCR_FAILURES=0 turns it off. Both are nil when the store cannot be opened; a store error
// leaves the compacted output as it is.
func ccrKeep() (all, failed func(raw []byte) (string, bool)) {
	cfg, err := contextopt.LoadConfig()
	full := contextopt.CCREnabled(cfg, err, os.Getenv("DOWNSHIFT_CCR"))
	failures := contextopt.CCRFailuresEnabled(cfg, err, os.Getenv("DOWNSHIFT_CCR_FAILURES"), os.Getenv("DOWNSHIFT_CCR"))
	if !full && !failures {
		return nil, nil
	}
	store, err := ccr.Default()
	if err != nil {
		return nil, nil
	}
	put := func(raw []byte) (string, bool) {
		id, err := store.Put(raw)
		return id, err == nil
	}
	if full {
		return put, nil
	}
	return nil, put
}
