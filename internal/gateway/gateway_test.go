// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package gateway_test

import (
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift/internal/gateway"
	"github.com/tiagovilasboas/downshift/internal/gateway/ninerouter"
)

func TestGatewayRegistry(t *testing.T) {
	nine := ninerouter.New(time.Second)

	gateways := []gateway.Gateway{nine}
	if len(gateways) != 1 {
		t.Fatalf("expected 1 gateway, got %d", len(gateways))
	}

	if nine.Name() != "9router" {
		t.Errorf("nine.Name() = %q, want %q", nine.Name(), "9router")
	}

	client := gateway.Client(0)
	if client == nil || client.Timeout != 1500*time.Millisecond {
		t.Errorf("expected default timeout 1500ms, got %v", client)
	}
}
