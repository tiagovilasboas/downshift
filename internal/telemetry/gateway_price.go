// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import "strings"

// Gateway runs: a child the harness sent through an OpenRouter route. The run
// is ranked by the tier of its allowlisted row (see gatewayDirection). A
// gateway id no row names has no tier: it is never credited as saved, and the
// tier of the requested model is never assumed for it. No price enters.

// IsGatewayModel reports a "vendor/model" slug, the id an OpenRouter route
// reports. Native catalog ids never contain a slash.
func IsGatewayModel(id string) bool {
	vendor, model, ok := strings.Cut(id, "/")
	return ok && vendor != "" && model != "" && !strings.Contains(model, "/")
}
