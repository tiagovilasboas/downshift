// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package accounting

import (
	"os"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/catalog"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// TestMain sets the token-direction catalog the way the composition root does
// at startup, so savings tests see the same ranking behaviour as runtime.
func TestMain(m *testing.M) {
	telemetry.DirectionCatalog = catalog.Load()
	os.Exit(m.Run())
}
