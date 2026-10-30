// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"time"

	"github.com/tiagovilasboas/downshift/internal/gateway"
	"github.com/tiagovilasboas/downshift/internal/gateway/ninerouter"
	"github.com/tiagovilasboas/downshift/internal/gateway/omniroute"
)

const gatewayUsage = `usage: downshift gateway [status] [--json]
       downshift gateway models
`

type gatewayStatusReport struct {
	Name          string `json:"name"`
	Type          string `json:"type"`
	Endpoint      string `json:"endpoint"`
	Active        bool   `json:"active"`
	Detail        string `json:"detail"`
	PrefixExample string `json:"prefix_example"`
	EnvHint       string `json:"env_hint"`
}

func runGateway(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		return runGatewayStatus(nil, stdout, stderr)
	}

	switch args[0] {
	case "status":
		return runGatewayStatus(args[1:], stdout, stderr)
	case "models":
		if len(args) > 1 {
			fmt.Fprint(stderr, gatewayUsage)
			return 2
		}
		return runGatewayModels(stdout)
	case "-h", "--help", "help":
		fmt.Fprint(stdout, gatewayUsage)
		return 0
	default:
		// If flag passed without subcommand (e.g. downshift gateway --json)
		if len(args) > 0 && args[0][0] == '-' {
			return runGatewayStatus(args, stdout, stderr)
		}
		fmt.Fprint(stderr, gatewayUsage)
		return 2
	}
}

func defaultGatewayInstances() []gateway.Gateway {
	timeout := 1000 * time.Millisecond
	omni := omniroute.New(timeout)
	nine := ninerouter.New(timeout)
	return []gateway.Gateway{
		&omni,
		&nine,
	}
}

func runGatewayStatus(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("gateway status", flag.ContinueOnError)
	fs.SetOutput(stderr)
	asJSON := fs.Bool("json", false, "emit JSON report")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() > 0 {
		fmt.Fprint(stderr, gatewayUsage)
		return 2
	}

	gateways := defaultGatewayInstances()
	ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancel()

	var reports []gatewayStatusReport
	for _, gw := range gateways {
		endpoint := gw.DefaultEndpoint()
		active, detail := gw.CheckHealth(ctx, endpoint)
		var envHint string
		if gw.Name() == "omniroute" {
			envHint = "export ANTHROPIC_BASE_URL=" + endpoint
		} else {
			envHint = "export ANTHROPIC_BASE_URL=" + endpoint
		}

		reports = append(reports, gatewayStatusReport{
			Name:          gw.Name(),
			Type:          gw.Name(),
			Endpoint:      endpoint,
			Active:        active,
			Detail:        detail,
			PrefixExample: fmt.Sprintf("%s/<provider>/<model>", gw.Name()),
			EnvHint:       envHint,
		})
	}

	if *asJSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(reports)
		return 0
	}

	fmt.Fprintln(stdout, "Gateway Companion Routers (local multi-provider failover proxies)")
	fmt.Fprintln(stdout)
	for _, rep := range reports {
		displayName := rep.Name
		if rep.Name == "omniroute" {
			displayName = "OmniRoute"
		}
		fmt.Fprintf(stdout, "%s:\n", displayName)
		statusStr := "unreachable"
		if rep.Active {
			statusStr = "active"
		}
		if rep.Detail != "" && !rep.Active {
			statusStr += fmt.Sprintf(" (%s)", rep.Detail)
		}
		fmt.Fprintf(stdout, "  Status:   %s\n", statusStr)
		fmt.Fprintf(stdout, "  Endpoint: %s\n", rep.Endpoint)
		fmt.Fprintf(stdout, "  Routing:  %s\n", rep.PrefixExample)
		fmt.Fprintf(stdout, "  Env hint: %s\n", rep.EnvHint)
		fmt.Fprintln(stdout)
	}
	fmt.Fprintln(stdout, "See docs/guides/companions.md for gateway configuration and failover setup.")
	return 0
}

func runGatewayModels(stdout io.Writer) int {
	fmt.Fprintln(stdout, `Gateway Model Routing

Downshift routes subagent spawns through local multi-provider gateways:

1. OmniRoute (default: http://localhost:20128/v1)
   Prefix format: omniroute/<provider>/<model-id>
   Examples:
     - omniroute/anthropic/claude-3-5-haiku-latest
     - omniroute/deepseek/deepseek-chat
     - omniroute/openai/gpt-4o-mini

2. 9router (default: http://localhost:3000)
   Prefix format: 9router/<provider>/<model-id>
   Examples:
     - 9router/anthropic/claude-3-5-haiku
     - 9router/openai/gpt-4o-mini
     - 9router/google/gemini-2.5-flash

Routing Mechanics:
  Downshift accepts gateway slugs as valid catalog entries, preserves
  the slug in PreToolUse hook rewrites, and leaves transport failover,
  credential pooling, and rate-limit retries to the local companion gateway.`)
	return 0
}
