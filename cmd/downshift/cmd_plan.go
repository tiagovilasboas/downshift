// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/tiagovilasboas/downshift-ai/internal/gateway"
	"github.com/tiagovilasboas/downshift-ai/internal/plan"
	"github.com/tiagovilasboas/downshift-ai/internal/playbook"
)

// runPlan is `downshift plan`: it validates the plan the lead LLM wrote on
// stdin, fills the defaults and prints a markdown table and next steps (or
// JSON). It calls no model. Workers route through the gateway when its switch
// is on, else "direct".
func runPlan(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("plan", flag.ContinueOnError)
	fs.SetOutput(stderr)
	asJSON := fs.Bool("json", false, "print the filled plan as JSON")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	data, err := io.ReadAll(io.LimitReader(stdin, 1<<20))
	if err != nil {
		fmt.Fprintln(stderr, "plan:", err)
		return 1
	}
	p, errs := plan.Resolve(data, planGateway(), planWorkerState(), playbookIDs())
	if len(errs) > 0 {
		fmt.Fprintf(stderr, "plan: %d problem(s):\n", len(errs))
		for _, e := range errs {
			fmt.Fprintln(stderr, "  -", e)
		}
		return 1
	}
	if *asJSON {
		out, err := p.JSON()
		if err != nil {
			fmt.Fprintln(stderr, "plan:", err)
			return 1
		}
		fmt.Fprintln(stdout, string(out))
		return 0
	}
	fmt.Fprint(stdout, p.Markdown())
	return 0
}

// planGateway reads the gateway switch; an unreadable one means off.
func planGateway() plan.Gateway {
	gs, _ := gateway.LoadState()
	return plan.Gateway{Enabled: gs.Enabled, Name: gs.Name}
}

// planWorkerState resolves the worker mode the same way the MCP server does.
func planWorkerState() plan.Workers {
	cwd, _ := os.Getwd()
	w := resolveWorkerMode(cwd, os.Getenv)
	return plan.Workers{Mode: w.Mode, Agents: w.Agents}
}

// playbookIDs lists the active playbook ids, falling back to the shipped ones.
func playbookIDs() []string {
	if dir, err := playbook.Dir(); err == nil {
		if sets, err := playbook.Load(dir); err == nil {
			ids := make([]string, 0, len(sets))
			for _, s := range sets {
				ids = append(ids, s.ID)
			}
			return ids
		}
	}
	return playbook.ShippedIDs()
}
