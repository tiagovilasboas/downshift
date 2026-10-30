// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package ninerouter provides the 9router gateway adapter: a local proxy that
// distributes LLM requests across multiple API providers with automatic 429 failover.
package ninerouter

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/tiagovilasboas/downshift/internal/gateway"
)

// Name is the canonical gateway name.
const Name = "9router"

// DefaultEndpoint is 9router's standard local proxy URL.
const DefaultEndpoint = "http://localhost:3000"

// Adapter implements gateway.Gateway for 9router.
type Adapter struct {
	Timeout time.Duration
}

// New returns a new 9router adapter.
func New(timeout time.Duration) Adapter {
	return Adapter{Timeout: timeout}
}

// Name implements gateway.Gateway.
func (Adapter) Name() string {
	return Name
}

// DefaultEndpoint implements gateway.Gateway.
func (Adapter) DefaultEndpoint() string {
	return DefaultEndpoint
}

// NormalizeID accepts model IDs used with 9router, allowing either "provider/model"
// format or standard vendor model identifiers configured through its routing table.
func (Adapter) NormalizeID(raw string) (string, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || strings.HasPrefix(trimmed, "/") || strings.HasSuffix(trimmed, "/") {
		return "", false
	}
	parts := strings.Split(trimmed, "/")
	if len(parts) > 2 {
		return "", false
	}
	return trimmed, true
}

// CheckHealth probes the 9router endpoint (e.g. GET /health or GET /) to check readiness.
func (a Adapter) CheckHealth(ctx context.Context, endpoint string) (bool, string) {
	if endpoint == "" {
		endpoint = DefaultEndpoint
	}
	reqURL := strings.TrimRight(endpoint, "/")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return false, err.Error()
	}

	client := gateway.Client(a.Timeout)
	resp, err := client.Do(req)
	if err != nil {
		return false, err.Error()
	}
	defer resp.Body.Close()

	if resp.StatusCode < 500 {
		return true, "online"
	}
	return false, resp.Status
}
