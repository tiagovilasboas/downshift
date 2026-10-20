// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/catalog"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// main sets the direction catalog at startup; tests stand in for it.
func TestMain(m *testing.M) {
	telemetry.DirectionCatalog = catalog.Load()
	os.Exit(m.Run())
}
