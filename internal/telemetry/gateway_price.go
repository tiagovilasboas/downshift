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

// IsGatewayModel reports a "vendor/model" slug, the id an OpenRouter route
// reports. Native catalog ids never contain a slash.
func IsGatewayModel(id string) bool {
	vendor, model, ok := strings.Cut(id, "/")
	return ok && vendor != "" && model != "" && !strings.Contains(model, "/")
}

// GatewayPriceStatus is the price source for a gateway model. No price snapshot
// is read any more, so every gateway run is unknown.
func GatewayPriceStatus(string) string {
	return GatewayPriceUnknown
}

// isUnattributedGateway reports a gateway run whose id has no allowlisted row,
// so it has no tier and no saving can be credited. A run whose id has a row is
// ranked by that tier like a native run, whatever the requested model is.
func isUnattributedGateway(dir, ran string, found bool) bool {
	return found && dir == TokenUnattributed && IsGatewayModel(ran) && !gatewayRowKnown(ran)
}
