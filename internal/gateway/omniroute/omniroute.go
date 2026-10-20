// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package omniroute is the OmniRoute gateway adapter: a self-hosted, OpenAI-
// compatible router whose ids are "provider/model". It implements only what the
// official OmniRoute pages confirm (docs/internal/tasks/2026-10-20-omniroute/
// research.md). Unconfirmed facts stay unknown: no price source, no retention
// evidence and no public model list. The composition root registers it.
package omniroute

import (
	"context"
	"strings"

	"github.com/tiagovilasboas/downshift/internal/gateway"
)

// Name is the registry key and the catalog "provider" value of this gateway.
const Name = "omniroute"

// routingAlias is the reserved provider name OmniRoute uses for routing
// aliases (auto, auto/coding). They select a model at request time, so they are
// not model ids and never canonical.
const routingAlias = "auto"

// Snapshot returns the list price of a canonical model in USD per million input
// and output tokens. The adapter does no I/O for prices. A nil Snapshot prices
// nothing, which is the state until an OmniRoute price source is confirmed.
type Snapshot func(model string) (inPerM, outPerM float64, ok bool)

// Gateway implements gateway.Gateway for OmniRoute.
type Gateway struct {
	Prices Snapshot
}

// New returns the OmniRoute gateway that prices from snapshot.
func New(snapshot Snapshot) Gateway {
	return Gateway{Prices: snapshot}
}

// Name implements gateway.Gateway.
func (Gateway) Name() string { return Name }

// NormalizeID accepts OmniRoute's "provider/model" form: exactly one slash with
// both parts present, and not a routing alias. The id is already canonical, so
// it is returned as is. The provider part is a route name, not a vendor; no
// mapping to a vendor is made.
func (Gateway) NormalizeID(raw string) (string, bool) {
	provider, model, ok := strings.Cut(raw, "/")
	if !ok || provider == "" || model == "" || strings.Contains(model, "/") {
		return "", false
	}
	if provider == routingAlias {
		return "", false
	}
	return raw, true
}

// Price implements gateway.Gateway from the supplied snapshot.
func (g Gateway) Price(canonical string) (float64, float64, bool) {
	if g.Prices == nil {
		return 0, 0, false
	}
	return g.Prices(canonical)
}

// Privacy implements gateway.Gateway. No official page states per-provider
// retention, so every model is unknown.
func (Gateway) Privacy(string) gateway.PrivacyInfo {
	return gateway.PrivacyInfo{Retention: gateway.RetentionUnknown, Source: "omniroute: no per-provider retention evidence in official pages (2026-10-20)"}
}

// ListModels implements gateway.Gateway. The model list is served by each
// instance, and whether it needs a key is not confirmed for a public list, so
// nothing is fetched.
func (Gateway) ListModels(context.Context) ([]gateway.ModelInfo, error) {
	return nil, gateway.ErrUnsupported
}
