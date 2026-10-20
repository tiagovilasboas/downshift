// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import "strings"

// Gateway runs: a child the harness sent through an OpenRouter route. Its
// usage is priced from the model that ran, read from the OpenRouter public
// list snapshot the operator opted in to. Without a snapshot price the run is
// unknown: it is never credited as saved, and the tier of the requested model
// is never assumed for it.

// Price sources recorded in Event.GatewayPrice.
const (
	GatewayPriceSnapshot = "snapshot"
	GatewayPriceUnknown  = "unknown"
)

// GatewayPriceLookup returns the list price of an OpenRouter model in USD per
// million input and output tokens, from the snapshot. ok is false when the
// snapshot has no price for it. The composition root sets it; nil means no
// snapshot is read, so every gateway run is unknown.
var GatewayPriceLookup func(model string) (inM, outM float64, ok bool)

// IsGatewayModel reports a "vendor/model" slug, the id an OpenRouter route
// reports. Native catalog ids never contain a slash.
func IsGatewayModel(id string) bool {
	vendor, model, ok := strings.Cut(id, "/")
	return ok && vendor != "" && model != "" && !strings.Contains(model, "/")
}

// GatewayPriceStatus is the price source for a gateway model: snapshot when the
// lookup prices it, unknown otherwise.
func GatewayPriceStatus(model string) string {
	if GatewayPriceLookup == nil {
		return GatewayPriceUnknown
	}
	if _, _, ok := GatewayPriceLookup(model); ok {
		return GatewayPriceSnapshot
	}
	return GatewayPriceUnknown
}

// gatewayDirection places a usage record that ran on a gateway model. The run
// is priced from what ran (the snapshot price) against the requested model's
// catalog price, by input plus output per million. Anything missing is
// unattributed: never a saving, never a tier guess.
func gatewayDirection(usage Event, requested, ran string) string {
	if usage.GatewayPrice != GatewayPriceSnapshot || GatewayPriceLookup == nil || DirectionCatalog == nil {
		return TokenUnattributed
	}
	inM, outM, ok := GatewayPriceLookup(ran)
	if !ok {
		return TokenUnattributed
	}
	req, ok := DirectionCatalog.LookupByID(usage.Harness, requested)
	if !ok || req.InputM <= 0 || req.OutputM <= 0 {
		return TokenUnattributed
	}
	ranCost, reqCost := inM+outM, req.InputM+req.OutputM
	switch {
	case ranCost < reqCost:
		return TokenCheaper
	case ranCost > reqCost:
		return TokenPricier
	}
	return TokenUnchanged
}

// isUnpricedGateway reports a gateway run that a saving could not be credited
// for because its price is unknown, on an applied rewrite with a linked decision.
func isUnpricedGateway(dir, ran string, found bool) bool {
	return found && dir == TokenUnattributed && IsGatewayModel(ran)
}
