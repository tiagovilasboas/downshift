// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/tiagovilasboas/downshift-ai/internal/gateway"
	"github.com/tiagovilasboas/downshift-ai/internal/routing/policy"
)

// gatewayWorkerEnv is the MCP delegate hook: the env a worker of the agent gets
// from the gateway switch. A missing or unreadable switch means no env.
func gatewayWorkerEnv(agent string) []string {
	st, err := gateway.LoadState()
	if err != nil {
		return nil
	}
	return gateway.WorkerEnv(st, agent)
}

func gatewayStateLine(st gateway.State) string {
	if st.Retired {
		return fmt.Sprintf("Delegate workers: gateway off (gateway.json names %q, which is no longer supported; run `downshift gateway on %s`)", st.Name, gateway.SupportedName)
	}
	if !st.Enabled {
		return "Delegate workers: gateway off"
	}
	return fmt.Sprintf("Delegate workers: routed through %s at %s (native subagents are not routed)", st.Name, st.Endpoint)
}

func runGatewayOn(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("gateway on", flag.ContinueOnError)
	fs.SetOutput(stderr)
	endpoint := fs.String("endpoint", "", "gateway endpoint URL (default: the gateway's own)")
	// Accept the name before or after the flag.
	var name string
	rest := args
	if len(rest) > 0 && len(rest[0]) > 0 && rest[0][0] != '-' {
		name, rest = rest[0], rest[1:]
	}
	if err := fs.Parse(rest); err != nil {
		return 2
	}
	if fs.NArg() > 1 || (name != "" && fs.NArg() > 0) {
		fmt.Fprint(stderr, gatewayUsage)
		return 2
	}
	if name == "" && fs.NArg() == 1 {
		name = fs.Arg(0)
	}
	if name == "" {
		name = "9router"
	}
	var gw gateway.Gateway
	for _, g := range defaultGatewayInstances() {
		if g.Name() == name {
			gw = g
		}
	}
	if gw == nil {
		fmt.Fprintf(stderr, "gateway on: unknown gateway %q (known: 9router)\n", name)
		return 2
	}
	ep := *endpoint
	if ep == "" {
		ep = gw.DefaultEndpoint()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancel()
	if ok, detail := gw.CheckHealth(ctx, ep); ok {
		fmt.Fprintf(stdout, "%s at %s is reachable\n", name, ep)
	} else {
		fmt.Fprintf(stderr, "warning: %s at %s is not reachable (%s); enabling anyway\n", name, ep, detail)
	}
	st := gateway.State{Enabled: true, Name: name, Endpoint: ep}
	if err := gateway.SaveState(st); err != nil {
		fmt.Fprintln(stderr, "gateway on:", err)
		return 1
	}
	fmt.Fprintln(stdout, gatewayStateLine(st))
	if root, _ := os.Getwd(); resolveWorkerMode(root, os.Getenv).Mode == policy.WorkersOff {
		fmt.Fprintf(stderr, "warning: %s; fix: %s\n", gatewayIdleMessage, workersFixSetup)
	}
	return 0
}

func runGatewayOff(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 {
		fmt.Fprint(stderr, gatewayUsage)
		return 2
	}
	st, err := gateway.LoadState()
	if err != nil {
		st = gateway.State{}
	}
	st.Enabled = false
	if err := gateway.SaveState(st); err != nil {
		fmt.Fprintln(stderr, "gateway off:", err)
		return 1
	}
	fmt.Fprintln(stdout, gatewayStateLine(st))
	return 0
}
