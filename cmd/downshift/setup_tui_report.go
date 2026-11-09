// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/tiagovilasboas/downshift-ai/internal/console/setupwizard"
	"github.com/tiagovilasboas/downshift-ai/internal/console/shared/theme"
	"github.com/tiagovilasboas/downshift-ai/internal/gateway/ninerouter"
	"github.com/tiagovilasboas/downshift-ai/internal/setup"
)

// renderSetupDone prints what follows a successful wizard apply: the gateway
// probe, the smoke check of the wired adapters, the equivalent command and the
// next steps. The diff is not repeated; setupRun printed it once.
func renderSetupDone(o setup.Options, sel setupwizard.Selection) {
	probeAndRenderGateways(o.Out)
	if targets, code := setup.Select(o); code == 0 {
		renderTUISmokeCheck(o.Out, targets)
	}
	writeSetupSummary(o.Out, theme.New(), sel)
}

// presetNames are the short names of wizardPresets, for the summary.
var presetNames = map[string]string{"essential": "Essential", "complete": "Complete", "custom": "Custom"}

// writeSetupSummary is the closing block of a wizard run: what was set up,
// the same setup as one command, and the three next steps.
func writeSetupSummary(w io.Writer, th theme.Theme, sel setupwizard.Selection) {
	label := func(s string) string { return th.Muted.Render(fmt.Sprintf("  %-11s ", s)) }
	fmt.Fprintf(w, "\n%s %s\n", th.OK.Render(theme.SymOK), th.Bold.Render("Downshift is set up"))
	fmt.Fprintf(w, "%s%s\n", label("Harnesses"), strings.Join(sel.Harnesses, ", "))
	if name := presetNames[sel.Preset]; name != "" {
		fmt.Fprintf(w, "%s%s\n", label("Profile"), name)
	}
	fmt.Fprintf(w, "%s%s\n", label("Components"), strings.Join(sel.Components, ", "))
	fmt.Fprintf(w, "\n  %s\n    %s\n", th.Muted.Render("The same setup without the wizard:"),
		th.Accent.Render(setupwizard.EquivalentCommand(sel, selectionArgs)))
	fmt.Fprintf(w, "\n  %s\n", th.Bold.Render("Next"))
	steps := [][2]string{
		{"", "Open a new session of your harness."},
		{"downshift doctor", "checks every connection"},
		{"downshift stats", "shows the tokens you save"},
	}
	for i, st := range steps {
		if st[0] == "" {
			fmt.Fprintf(w, "    %d. %s\n", i+1, st[1])
			continue
		}
		fmt.Fprintf(w, "    %d. %s  %s\n", i+1, th.Accent.Render(fmt.Sprintf("%-17s", st[0])), th.Muted.Render(st[1]))
	}
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
