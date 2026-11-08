// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/tiagovilasboas/downshift-ai/internal/setup"
)

// checkCrewRegistry warns when the informational list still has a Downshift
// row. Harnesses with no Registry are skipped.
func checkCrewRegistry(add func(id, status, detail, fix string), env doctorEnv, harness string, w doctorWiring) {
	if w.Registry == "" {
		return
	}
	path := filepath.Join(env.Home, filepath.FromSlash(w.Registry))
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return
	}
	if err != nil {
		add("crew registry", doctorWarn, fmt.Sprintf("cannot read %s: %v", w.Registry, err), "downshift setup "+harness)
		return
	}
	_, removed, err := setup.StripDownshiftCrewHooks(raw)
	if err != nil {
		add("crew registry", doctorWarn, fmt.Sprintf("%s is not valid JSON: %v", w.Registry, err), "downshift setup "+harness)
		return
	}
	if removed == 0 {
		return
	}
	add("crew registry", doctorWarn,
		fmt.Sprintf("%s has %d Downshift row(s); that list does not enforce policy and duplicates the native hook", w.Registry, removed),
		"downshift setup "+harness)
}
