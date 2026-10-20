// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package catalog

import (
	"github.com/tiagovilasboas/downshift/internal/gateway"
	"github.com/tiagovilasboas/downshift/internal/gateway/openrouter"
)

// Gateway rows are recognised through the registry; tests register the
// OpenRouter adapter the way cmd/downshift does at startup.
func init() {
	gateway.Register(openrouter.New(nil))
}
