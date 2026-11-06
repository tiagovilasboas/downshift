// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"io"

	"github.com/tiagovilasboas/downshift/internal/catalog"
)

// catalogLoader is the catalog entry point; tests replace it.
var catalogLoader = catalog.Load

// loadCatalogFailOpen loads the catalog for a hook. A panic anywhere in the
// load (a defect in the embedded data or a hostile override) is reported on w
// and returned as ok=false, so the caller can allow the spawn unchanged
// instead of letting the runtime exit 2, which a harness treats as a deny.
func loadCatalogFailOpen(w io.Writer) (c *catalog.Catalog, ok bool) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(w, "downshift: internal error loading the catalog (%v), spawn allowed unchanged\n", r)
			c, ok = nil, false
		}
	}()
	return catalogLoader(), true
}
