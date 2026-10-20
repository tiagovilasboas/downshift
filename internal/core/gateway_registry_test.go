// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core_test

import (
	"context"

	"github.com/tiagovilasboas/downshift/internal/catalog"
	"github.com/tiagovilasboas/downshift/internal/gateway"
)

// stubGateway registers the allowlist's gateway name so the catalog reads its
// rows as gateway rows. Core tests use the port only; no adapter is imported.
type stubGateway struct{}

func (stubGateway) Name() string { return catalog.AllowlistProvider }

func (stubGateway) NormalizeID(raw string) (string, bool) { return raw, raw != "" }

func (stubGateway) Price(string) (float64, float64, bool) { return 0, 0, false }

func (stubGateway) Privacy(string) gateway.PrivacyInfo {
	return gateway.PrivacyInfo{Retention: gateway.RetentionUnknown, Source: "stub"}
}

func (stubGateway) ListModels(context.Context) ([]gateway.ModelInfo, error) {
	return nil, gateway.ErrUnsupported
}

func init() {
	gateway.Register(stubGateway{})
}
