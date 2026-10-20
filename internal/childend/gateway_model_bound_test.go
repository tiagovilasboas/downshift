// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package childend

import (
	"testing"

	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

func TestMarkGatewayRunBoundsUntrustedSlug(t *testing.T) {
	var ev telemetry.Event
	markGatewayRun(&ev, "vendor/mo\ndel")
	if ev.ToModel != "unknown" {
		t.Fatalf("ToModel = %q, want unknown for a slug with a newline", ev.ToModel)
	}
}

func TestMarkGatewayRunKeepsValidSlug(t *testing.T) {
	var ev telemetry.Event
	markGatewayRun(&ev, "vendor/model-1")
	if ev.ToModel != "vendor/model-1" {
		t.Fatalf("ToModel = %q, want the valid slug unchanged", ev.ToModel)
	}
}
