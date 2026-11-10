// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"

	"github.com/tiagovilasboas/downshift-ai/internal/contextopt"
	"github.com/tiagovilasboas/downshift-ai/internal/execwrap"
)

// checkCompactorMode reports how large outputs are compacted: balanced (cut
// with the raw output kept for retrieve) or lossless, and why.
func checkCompactorMode(rep *doctorReport) {
	cfg, err := contextopt.LoadConfig()
	keep := contextopt.CCREnabled(cfg, err, os.Getenv("DOWNSHIFT_CCR")) ||
		contextopt.CCRFailuresEnabled(cfg, err, os.Getenv("DOWNSHIFT_CCR_FAILURES"), os.Getenv("DOWNSHIFT_CCR"))
	switch {
	case !balancedOn():
		rep.add(doctorCheck{ID: "compactor mode", Status: doctorOK, Detail: "lossless (balanced off by " + execwrap.BalancedEnv + "=0 or \"balanced\": false)"})
	case !keep:
		rep.add(doctorCheck{ID: "compactor mode", Status: doctorWarn, Detail: "lossless: balanced needs CCR to keep the raw output, and CCR is off",
			Fix: "unset DOWNSHIFT_CCR or remove \"ccr\": false from the config"})
	default:
		rep.add(doctorCheck{ID: "compactor mode", Status: doctorOK, Detail: "balanced: outputs over 16 KB are cut, full output via downshift retrieve"})
	}
}
