// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core

// GatewayLiveResolver exposes the gateway rows the owner marked live. It is
// read by telemetry to attribute observed tokens to a gateway. It never makes a
// row a rewrite target: a model is written only when the session lists it.
type GatewayLiveResolver interface {
	LiveGateways() []Model
}
