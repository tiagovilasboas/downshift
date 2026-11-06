// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/tiagovilasboas/downshift/internal/console/setupwizard"
	"github.com/tiagovilasboas/downshift/internal/gateway/ninerouter"
	"github.com/tiagovilasboas/downshift/internal/setup"
)

// renderSetupDone prints what follows a successful wizard apply: the gateway
// probe, the smoke check of the wired adapters, the equivalent command and the
// next steps. The diff is not repeated; setupRun printed it once.
func renderSetupDone(o setup.Options, sel setupwizard.Selection) {
	probeAndRenderGateways(o.Out)
	if targets, code := setup.Select(o); code == 0 {
		renderTUISmokeCheck(o.Out, targets)
	}
	fmt.Fprintf(o.Out, "Equivalent command:\n  %s\n\n", setupwizard.EquivalentCommand(sel, selectionArgs))
	fmt.Fprintln(o.Out, "Setup completed. Next steps:")
	fmt.Fprintln(o.Out, "  - Open a new window or session of your harness.")
	fmt.Fprintln(o.Out, "  - Run 'downshift doctor' at any time to check the health of the connections.")
	fmt.Fprintln(o.Out, "  - Run 'downshift stats' to follow your token savings.")
}

func probeAndRenderGateways(out io.Writer) {
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	nine := ninerouter.New(150 * time.Millisecond)

	nineOK, _ := nine.CheckHealth(ctx, nine.DefaultEndpoint())

	var detected []string
	if nineOK {
		detected = append(detected, fmt.Sprintf("9router (%s)", nine.DefaultEndpoint()))
	}
	if len(detected) > 0 {
		fmt.Fprintf(out, "Active companion gateways detected: %s\n\n", strings.Join(detected, ", "))
	}
}
