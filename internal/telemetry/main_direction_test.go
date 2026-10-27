// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"os"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/catalog"
	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/telemetry/catalogtier"
)

// The composition root sets DirectionCatalog; tests stand in for it.
func TestMain(m *testing.M) {
	DirectionCatalog = catalogtier.Of(catalog.Load())
	for _, n := range core.AllHarnessCaps() {
		DeclaredHarnesses = append(DeclaredHarnesses, HarnessSeam{Harness: n.Harness, ChildEndEvent: n.Caps.ChildEndEvent, UsageSource: n.Caps.UsageSource})
	}
	os.Exit(m.Run())
}
