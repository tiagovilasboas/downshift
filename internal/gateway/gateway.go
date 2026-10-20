// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package gateway is the port between Downshift and a model gateway: a service
// that serves many vendors' models behind one API and one id scheme. Core and
// telemetry import only this package. Each gateway is an adapter under
// internal/gateway/<name>, registered by the composition root, so no router
// name appears in shared code.
package gateway

import (
	"context"
	"errors"
	"sort"
	"sync"
	"time"
)

// ErrUnsupported is returned by ListModels from a gateway that cannot list
// its models.
var ErrUnsupported = errors.New("gateway: model listing not supported")

// Retention values for PrivacyInfo.Retention.
const (
	RetentionZero     = "zero"     // provider keeps no prompts or completions
	RetentionUnknown  = "unknown"  // no evidence either way; the default
	RetentionRetained = "retained" // provider is known to retain data
)

// PrivacyInfo is what a gateway can say about where a model's data goes.
// Source names the evidence; it is never empty.
type PrivacyInfo struct {
	Retention string
	Source    string
}

// ModelInfo is one model a gateway lists. ID is the canonical "vendor/model"
// form. InputM and OutputM are USD per million tokens; nil means unknown.
type ModelInfo struct {
	ID        string
	Display   string
	CreatedAt time.Time
	InputM    *float64
	OutputM   *float64
}

// Gateway is one router. Implementations hold no policy: they translate ids,
// report prices from the snapshot they were given, and list models.
type Gateway interface {
	// Name is the stable registry key and the value a catalog row uses in its
	// "provider" field.
	Name() string
	// NormalizeID maps a gateway-native id to the canonical "vendor/model"
	// form. ok is false when raw is not a model id this gateway serves.
	NormalizeID(raw string) (canonical string, ok bool)
	// Price returns the list price of a canonical id from the gateway's
	// snapshot. known is false when the snapshot has no price for it.
	Price(canonical string) (inPerM, outPerM float64, known bool)
	// Privacy reports the data-retention evidence for a canonical id.
	Privacy(canonical string) PrivacyInfo
	// ListModels returns the gateway's public model list. A gateway that
	// cannot list returns ErrUnsupported.
	ListModels(ctx context.Context) ([]ModelInfo, error)
}

var (
	mu       sync.RWMutex
	registry = map[string]Gateway{}
)

// Register adds g under its Name, replacing any earlier registration with the
// same name. Call it once at the composition root before any catalog load.
func Register(g Gateway) {
	mu.Lock()
	defer mu.Unlock()
	registry[g.Name()] = g
}

// Lookup returns the gateway registered under name.
func Lookup(name string) (Gateway, bool) {
	mu.RLock()
	defer mu.RUnlock()
	g, ok := registry[name]
	return g, ok
}

// Names lists the registered gateway names in sorted order.
func Names() []string {
	mu.RLock()
	defer mu.RUnlock()
	names := make([]string, 0, len(registry))
	for n := range registry {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// PriceOf prices a canonical model from the first registered gateway (in Names
// order) whose snapshot knows it. It is the lookup telemetry uses; it returns
// false when no registered gateway prices the model.
func PriceOf(canonical string) (inPerM, outPerM float64, known bool) {
	for _, n := range Names() {
		g, ok := Lookup(n)
		if !ok {
			continue
		}
		if in, out, ok := g.Price(canonical); ok {
			return in, out, true
		}
	}
	return 0, 0, false
}
