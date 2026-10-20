// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package corerouter

import (
	"github.com/tiagovilasboas/downshift/internal/catalog"
	"github.com/tiagovilasboas/downshift/internal/replay"
)

// Comparer prices models with the catalog's own ratio, the same one the router
// and the report use, so a replayed saving and a reported saving agree.
type Comparer struct {
	Cat *catalog.Catalog
}

// Known reports whether the catalog resolves the model id for Claude Code.
func (c Comparer) Known(model string) bool {
	_, ok := c.Cat.LookupByID(Harness, model)
	return ok
}

// Compare returns how to relates to from, by list price.
func (c Comparer) Compare(from, to string) replay.Direction {
	a, okA := c.Cat.LookupByID(Harness, from)
	b, okB := c.Cat.LookupByID(Harness, to)
	if !okA || !okB {
		return replay.Unknown
	}
	if a.ID == b.ID {
		return replay.Same
	}
	if c.Cat.SavingsRatio(a, b) > 0 {
		return replay.Cheaper
	}
	if c.Cat.SavingsRatio(b, a) > 0 {
		return replay.Pricier
	}
	return replay.Same
}
