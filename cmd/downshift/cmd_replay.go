// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/tiagovilasboas/downshift/internal/catalog"
	"github.com/tiagovilasboas/downshift/internal/replay"
	"github.com/tiagovilasboas/downshift/internal/replay/ccsource"
	"github.com/tiagovilasboas/downshift/internal/replay/corerouter"
)

// runReplay re-decides real Claude Code spawns under each policy and prints the
// tokens each one would have moved. Offline: no model call, no network, and no
// prompt is printed or written.
func runReplay(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("replay", flag.ContinueOnError)
	fs.SetOutput(stderr)
	projects := fs.String("projects", "", "Claude Code projects directory (default ~/.claude/projects)")
	policies := fs.String("policies", "current,nb_downshift", "comma-separated policy names: current, nb_downshift, explicit_upshift, or nb_downshift+explicit_upshift")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	root := *projects
	if root == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Fprintln(stderr, "replay: no home directory:", err)
			return 1
		}
		root = filepath.Join(home, ".claude", "projects")
	}
	pols, err := parsePolicies(*policies)
	if err != nil {
		fmt.Fprintln(stderr, "replay:", err)
		return 2
	}
	cat := catalog.Load()
	router := corerouter.Router{Res: cat, Models: claudeModels(cat)}
	cmp := corerouter.Comparer{Cat: cat}
	rep, err := replay.Evaluate(context.Background(), ccsource.New(root, cmp), router, cmp, pols)
	if err != nil {
		fmt.Fprintln(stderr, "replay:", err)
		return 1
	}
	printReplay(stdout, root, rep)
	return 0
}

func parsePolicies(spec string) ([]replay.Policy, error) {
	known := map[string]map[string]string{
		"current":                       {},
		"nb_downshift":                  {corerouter.KnobNBDownshift: "1"},
		"explicit_upshift":              {corerouter.KnobExplicitUpshift: "1"},
		"nb_downshift+explicit_upshift": {corerouter.KnobNBDownshift: "1", corerouter.KnobExplicitUpshift: "1"},
	}
	var out []replay.Policy
	for _, name := range strings.Split(spec, ",") {
		name = strings.TrimSpace(name)
		knobs, ok := known[name]
		if !ok {
			return nil, fmt.Errorf("unknown policy %q", name)
		}
		out = append(out, replay.Policy{Name: name, Knobs: knobs})
	}
	return out, nil
}

// claudeModels lists the ids the catalog knows for Claude Code: the session the
// replay writes into.
func claudeModels(cat *catalog.Catalog) []string {
	var ids []string
	for _, e := range cat.Entries() {
		if e.Harness == corerouter.Harness {
			ids = append(ids, e.ID)
		}
	}
	return ids
}

func printReplay(w io.Writer, root string, rep replay.Report) {
	fmt.Fprintf(w, "Replay of real Claude Code spawns from %s\n", root)
	fmt.Fprintf(w, "Evaluated %d spawns. Offline counterfactual on list prices; no model call, no prompt is printed.\n\n", rep.Evaluated)
	for _, s := range rep.SkipLines() {
		fmt.Fprintf(w, "  skipped %-28s %d\n", s.Reason, s.Count)
	}
	fmt.Fprintln(w)
	fmt.Fprintf(w, "%-30s %8s %8s %18s %18s %10s\n", "policy", "agree", "held", "cheaper tokens", "pricier tokens", "pricier n")
	for _, r := range append([]replay.PolicyResult{rep.Actual}, rep.Policies...) {
		fmt.Fprintf(w, "%-30s %8s %8d %18d %18d %10d\n", r.Policy.Name, pct(r.Agree, rep.Evaluated), r.Held,
			r.Cheaper.Tokens.Total(), r.Pricier.Tokens.Total(), r.Pricier.Spawns)
	}
}

func pct(n, of int) string {
	if of == 0 {
		return "n/a"
	}
	return fmt.Sprintf("%.1f%%", 100*float64(n)/float64(of))
}
