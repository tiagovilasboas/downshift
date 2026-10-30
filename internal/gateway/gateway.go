// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package gateway provides router-agnostic abstraction for multi-provider AI
// proxy gateways (such as OmniRoute and 9router).
package gateway

import (
	"context"
	"net/http"
	"time"
)

// Gateway is the interface implemented by proxy routers.
type Gateway interface {
	// Name returns the canonical identifier of the gateway.
	Name() string
	// DefaultEndpoint returns the standard local URL the gateway listens on.
	DefaultEndpoint() string
	// NormalizeID validates and normalizes a model ID declared through the gateway.
	NormalizeID(raw string) (string, bool)
	// CheckHealth probes the gateway endpoint to check if it is active.
	CheckHealth(ctx context.Context, endpoint string) (bool, string)
}

// Client returns a standard HTTP client configured with a short timeout for healthchecks.
func Client(timeout time.Duration) *http.Client {
	if timeout <= 0 {
		timeout = 1500 * time.Millisecond
	}
	return &http.Client{
		Timeout: timeout,
	}
}
