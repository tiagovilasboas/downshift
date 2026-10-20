// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"sync"

	"github.com/tiagovilasboas/downshift/internal/discovery"
	"github.com/tiagovilasboas/downshift/internal/gateway"
	"github.com/tiagovilasboas/downshift/internal/gateway/openrouter"
)

// registerGateways is the composition root for gateway adapters. Each adapter
// is registered here and nowhere else; shared layers see only the port.
func registerGateways() {
	gateway.Register(openrouter.New(gatewaySnapshotPrice()))
}

// gatewaySnapshotPrice reads gateway list prices from the discovery cache,
// once per process. It only reads: the snapshot is written by
// `downshift models discover` when the operator opted in. A missing or invalid
// cache prices nothing.
func gatewaySnapshotPrice() openrouter.Snapshot {
	var once sync.Once
	var cache discovery.File
	return func(model string) (float64, float64, bool) {
		once.Do(func() {
			if path, err := discovery.Path(); err == nil {
				cache = discovery.Read(path)
			}
		})
		return discovery.SnapshotPrice(cache, model)
	}
}
