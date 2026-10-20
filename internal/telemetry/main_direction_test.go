// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"os"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/catalog"
)

// The composition root sets DirectionCatalog; tests stand in for it.
func TestMain(m *testing.M) {
	DirectionCatalog = catalog.Load()
	os.Exit(m.Run())
}
