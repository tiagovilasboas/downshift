// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package omniroute provides the OmniRoute gateway adapter: a self-hosted,
// OpenAI-compatible AI proxy whose model IDs are formatted as "provider/model".
package omniroute

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/tiagovilasboas/downshift/internal/gateway"
)

// Name is the canonical gateway name.
const Name = "omniroute"

// DefaultEndpoint is OmniRoute's default local URL.
const DefaultEndpoint = "http://localhost:20128/v1"

// routingAlias is the reserved prefix OmniRoute uses for dynamic routing aliases.
const routingAlias = "auto"

// Adapter implements gateway.Gateway for OmniRoute.
type Adapter struct {
	Timeout time.Duration
}

// New returns a new OmniRoute adapter.
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

// NormalizeID accepts OmniRoute's "provider/model" format: exactly one slash with
// both parts present, rejecting standalone routing aliases and multi-slash IDs.
func (Adapter) NormalizeID(raw string) (string, bool) {
	provider, model, ok := strings.Cut(raw, "/")
	if !ok || provider == "" || model == "" || strings.Contains(model, "/") {
		return "", false
	}
	if provider == routingAlias && !strings.Contains(model, "-") && !strings.Contains(model, "_") {
		// Pure dynamic aliases like "auto/coding" are allowed if structured, but pure "auto" is rejected
		if model == "" {
			return "", false
		}
	}
	return raw, true
}

// CheckHealth probes the endpoint (or default) to verify if OmniRoute is running.
func (a Adapter) CheckHealth(ctx context.Context, endpoint string) (bool, string) {
	if endpoint == "" {
		endpoint = DefaultEndpoint
	}
	reqURL := strings.TrimRight(endpoint, "/") + "/models"
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

	// OmniRoute returns 200 on /models, or 401 if REQUIRE_API_KEY=true; both confirm it is alive.
	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusUnauthorized {
		return true, "online"
	}
	return false, resp.Status
}
