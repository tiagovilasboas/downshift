// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"time"

	"github.com/tiagovilasboas/downshift-ai/internal/gateway"
	"github.com/tiagovilasboas/downshift-ai/internal/gateway/ninerouter"
)

// defaultDoctorGateways returns the standard companion gateways checked by doctor.
func defaultDoctorGateways() []gateway.Gateway {
	timeout := 500 * time.Millisecond
	nine := ninerouter.New(timeout)
	return []gateway.Gateway{
		&nine,
	}
}

// checkGateways probes local gateway companions (9router).
// Active gateways are reported as ok; absent gateways only show in verbose mode
// since they are optional companion routers.
func checkGateways(rep *doctorReport, env doctorEnv) {
	if env.Gateways == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	for _, gw := range env.Gateways() {
		endpoint := gw.DefaultEndpoint()
		ok, _ := gw.CheckHealth(ctx, endpoint)
		if ok {
			rep.add(doctorCheck{
				ID:     "gateway",
				Status: doctorOK,
				Detail: fmt.Sprintf("%s active at %s (prefix: %s/<provider>/<model>)", gw.Name(), endpoint, gw.Name()),
			})
		} else if env.Verbose {
			rep.add(doctorCheck{
				ID:     "gateway",
				Status: doctorOK,
				Detail: fmt.Sprintf("%s not detected at %s (optional companion, docs/guides/companions.md)", gw.Name(), endpoint),
			})
		}
	}
}
