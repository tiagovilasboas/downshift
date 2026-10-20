// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import "strings"

// Gateway runs: a child the harness sent through an OpenRouter route. The run
// is ranked by the tier of its allowlisted row (see gatewayDirection). The
// price snapshot only labels the run's price source. A gateway id no row names
// is unknown: it is never credited as saved, and the tier of the requested
// model is never assumed for it.

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

// isUnattributedGateway reports a gateway run whose id has no allowlisted row,
// so it has no tier and no saving can be credited. A run whose id has a row is
// ranked by that tier like a native run, whatever the requested model is.
func isUnattributedGateway(dir, ran string, found bool) bool {
	return found && dir == TokenUnattributed && IsGatewayModel(ran) && !gatewayRowKnown(ran)
}
