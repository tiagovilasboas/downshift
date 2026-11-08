// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/tiagovilasboas/downshift-ai/internal/gateway"
	"github.com/tiagovilasboas/downshift-ai/internal/routing/policy"
)

const (
	workersFixSetup  = "downshift setup --workers read"
	workersFixPolicy = "or set orchestration.workers in ~/.downshift/policy.json"
	// gatewayIdleMessage is shown by doctor and by `gateway on` when the
	// gateway is enabled while no worker can route through it.
	gatewayIdleMessage = "gateway is on but workers are off, so nothing routes through it"
)

// workersDetail renders a resolved worker mode as one doctor detail line.
func workersDetail(w policy.Workers) string {
	src := "source " + w.Source
	agents := strings.Join(w.Agents, ", ")
	var d string
	switch w.Mode {
	case policy.WorkersRead:
		d = fmt.Sprintf("read (%s) · write off · %s", agents, src)
	case policy.WorkersWrite:
		d = fmt.Sprintf("read+write (%s) · %s", agents, src)
	default:
		d = "off · " + src
	}
	if len(w.Notes) > 0 {
		d += " · " + strings.Join(w.Notes, "; ")
	}
	return d
}

// checkWorkers reports the resolved worker mode and, when workers are off
// while the gateway is enabled, the idle-gateway warning.
func checkWorkers(rep *doctorReport, env doctorEnv) {
	root, _ := os.Getwd()
	getenv := env.Getenv
	if getenv == nil {
		getenv = os.Getenv
	}
	w := resolveWorkerMode(root, getenv)
	c := doctorCheck{ID: "workers", Status: doctorOK, Detail: workersDetail(w)}
	if w.Mode == policy.WorkersOff {
		c.Fix = workersFixSetup + " " + workersFixPolicy
	}
	rep.add(c)
	if w.Mode == policy.WorkersOff {
		if st, err := gateway.LoadState(); err == nil && st.Enabled {
			rep.add(doctorCheck{ID: "gateway", Status: doctorWarn, Detail: gatewayIdleMessage, Fix: workersFixSetup})
		}
	}
}
