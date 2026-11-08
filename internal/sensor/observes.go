// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package sensor

import "github.com/tiagovilasboas/downshift-ai/internal/core"

// Observes reports whether a tool result of this category feeds the context
// sensor. Read and search results are counted only as main-session work
// (internal/mainwork); the sensor measures shell and spawn output.
func Observes(c core.ToolCategory) bool {
	return c != core.CategoryRead && c != core.CategorySearch
}
