// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package gateway_test

import (
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift/internal/gateway"
	"github.com/tiagovilasboas/downshift/internal/gateway/ninerouter"
	"github.com/tiagovilasboas/downshift/internal/gateway/omniroute"
)

func TestGatewayRegistry(t *testing.T) {
	omni := omniroute.New(time.Second)
	nine := ninerouter.New(time.Second)

	gateways := []gateway.Gateway{omni, nine}
	if len(gateways) != 2 {
		t.Fatalf("expected 2 gateways, got %d", len(gateways))
	}

	if omni.Name() != "omniroute" {
		t.Errorf("omni.Name() = %q, want %q", omni.Name(), "omniroute")
	}
	if nine.Name() != "9router" {
		t.Errorf("nine.Name() = %q, want %q", nine.Name(), "9router")
	}

	client := gateway.Client(0)
	if client == nil || client.Timeout != 1500*time.Millisecond {
		t.Errorf("expected default timeout 1500ms, got %v", client)
	}
}
