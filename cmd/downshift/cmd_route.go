// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/routing/contract"
)

// routeSchemaVersion is the version of the `route --json` object.
const routeSchemaVersion = 1

// routeJSON is the stable `route --json` object. It holds ids, enums, tiers
// and numbers: no task text.
type routeJSON struct {
	SchemaVersion  int            `json:"schema_version"`
	Harness        string         `json:"harness"`
	RequestedModel string         `json:"requested_model"`
	SelectedModel  string         `json:"selected_model"`
	Tier           string         `json:"tier"`
	Effort         string         `json:"effort"`
	Verdict        string         `json:"verdict"`
	Reason         string         `json:"reason"`
	Applied        bool           `json:"applied"`
	Note           map[string]any `json:"note"`
}

// runRoute prints what Downshift would apply for one spawn. The orchestrator's
// choice is the request; declared policy and the tier hint apply on top. No
// prompt text is accepted.
func runRoute(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("route", flag.ContinueOnError)
	fs.SetOutput(stderr)
	harness := fs.String("harness", "", "harness id")
	model := fs.String("model", "", "requested model id")
	effort := fs.String("effort", "", "requested reasoning effort")
	tier := fs.String("tier", "", "tier hint: small, mid or frontier")
	agentType := fs.String("agent-type", "", "subagent type")
	dir := fs.String("dir", "", "project directory (default: current directory)")
	asJSON := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(stderr, "route: unexpected argument; it takes flags only (see downshift route --help)")
		return 2
	}
	if msg := routeFlagError(*harness, *effort, *tier); msg != "" {
		fmt.Fprintln(stderr, "route: "+msg)
		return 2
	}
	project := *dir
	if project == "" {
		project, _ = os.Getwd()
	}
	var cat core.Resolver
	if c, ok := loadCatalogFailOpen(stderr); ok && c != nil {
		cat = c
	}
	res, err := spawnApplier{cat: cat, now: time.Now}.Apply(context.Background(), ApplyInput{
		Harness: *harness, RequestedModel: *model, Effort: *effort,
		AgentType: *agentType, TierHint: *tier, ProjectDir: project,
	})
	if err != nil {
		fmt.Fprintln(stderr, "route:", err)
		return 1
	}
	if *asJSON {
		return writeRouteJSON(stdout, stderr, res)
	}
	writeRouteText(stdout, res)
	return 0
}

// routeFlagError names the first invalid flag value, or "" when all are valid.
func routeFlagError(harness, effort, tier string) string {
	if harness == "" {
		return "--harness is required; valid ids: " + strings.Join(routeHarnessIDs(), ", ")
	}
	if !slices.Contains(routeHarnessIDs(), harness) {
		return fmt.Sprintf("unknown harness %q; valid ids: %s", harness, strings.Join(routeHarnessIDs(), ", "))
	}
	if effort != "" && !slices.Contains([]string{"low", "medium", "high"}, effort) {
		return fmt.Sprintf("unknown effort %q; valid: low, medium, high", effort)
	}
	if tier != "" && !contract.Tier(tier).Valid() {
		return fmt.Sprintf("unknown tier %q; valid: small, mid, frontier", tier)
	}
	return ""
}

func routeHarnessIDs() []string {
	var ids []string
	for _, h := range core.AllHarnessCaps() {
		ids = append(ids, h.Harness)
	}
	return ids
}

func writeRouteJSON(stdout, stderr io.Writer, r ApplyResult) int {
	out := routeJSON{
		SchemaVersion: routeSchemaVersion, Harness: r.Harness,
		RequestedModel: r.RequestedModel, SelectedModel: r.SelectedModel,
		Tier: r.Tier, Effort: r.Effort, Verdict: r.Verdict, Reason: r.Reason,
		Applied: r.Applied, Note: r.Note,
	}
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		fmt.Fprintln(stderr, "route:", err)
		return 1
	}
	return 0
}

func writeRouteText(w io.Writer, r ApplyResult) {
	row := func(k, v string) {
		if v == "" {
			v = "-"
		}
		fmt.Fprintf(w, "%-10s %s\n", k, v)
	}
	row("harness", r.Harness)
	row("requested", r.RequestedModel)
	row("selected", r.SelectedModel)
	row("tier", r.Tier)
	row("effort", r.Effort)
	row("verdict", r.Verdict)
	row("applied", fmt.Sprint(r.Applied))
	row("reason", r.Reason)
}
