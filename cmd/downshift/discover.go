// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/tiagovilasboas/downshift/internal/discovery"
	"github.com/tiagovilasboas/downshift/internal/models"
)

// runModelsDiscover asks each harness which models the current account can use
// and writes the ranked result where hooks read it. It is the only place that
// touches the network or runs a harness CLI; hooks only read the cache.
func runModelsDiscover(catalog models.CatalogReader, args []string, w, errW io.Writer) int {
	cat, ok := catalog.(discovery.Catalog)
	if !ok {
		fmt.Fprintln(errW, "discover: catalog does not expose entry metadata")
		return 1
	}
	path, err := discovery.Path()
	if err != nil {
		fmt.Fprintf(errW, "discover: %v\n", err)
		return 1
	}
	return discoverWith(cat, discovery.Defaults(), path, args, w, errW)
}

func discoverWith(cat discovery.Catalog, sources []discovery.Source, cachePath string, args []string, w, errW io.Writer) int {
	fs := flag.NewFlagSet("models discover", flag.ContinueOnError)
	fs.SetOutput(errW)
	only := fs.String("harness", "", "comma-separated harness ids to discover (default: all with a source)")
	dry := fs.Bool("dry-run", false, "report without writing the cache")
	asJSON := fs.Bool("json", false, "print the result as JSON")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	want := map[string]bool{}
	for _, h := range strings.Split(*only, ",") {
		if h = strings.TrimSpace(h); h != "" {
			want[h] = true
		}
	}
	available := map[string]bool{}
	for _, s := range sources {
		available[s.Harness()] = true
	}
	for h := range want {
		if !available[h] {
			fmt.Fprintf(errW, "discover: no source for requested harness %s\n", h)
			return 2
		}
	}
	var picked []discovery.Source
	for _, s := range sources {
		if len(want) == 0 || want[s.Harness()] {
			picked = append(picked, s)
		}
	}
	if len(picked) == 0 {
		fmt.Fprintln(errW, "discover: no source for the requested harness")
		return 2
	}

	now := time.Now().UTC()
	outcomes := discovery.Run(context.Background(), picked, 20*time.Second)
	updates := map[string]discovery.Entry{}
	failed := 0
	for _, o := range outcomes {
		h := o.Source.Harness()
		if o.Err != nil {
			failed++
			if *asJSON {
				fmt.Fprintf(errW, "%s (%s): skipped: %v\n", h, o.Source.Name(), o.Err)
			} else {
				fmt.Fprintf(w, "%-12s %-28s skipped: %v\n", h, o.Source.Name(), o.Err)
			}
			continue
		}
		ranked := discovery.Order(h, o.Models, cat)
		updates[h] = discovery.Entry{Source: o.Source.Name(), FetchedAt: now, Models: o.Models, Ranked: ranked}
		if !*asJSON {
			printDiscovered(w, h, o.Source.Name(), o.Models, ranked)
		}
	}

	if *asJSON {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		_ = enc.Encode(discovery.File{Version: 1, Harnesses: updates})
	}
	if len(updates) == 0 {
		fmt.Fprintln(errW, "discover: nothing discovered; cache left unchanged")
		return 1
	}
	if *dry {
		if !*asJSON {
			fmt.Fprintln(w, "\ndry run: cache not written")
		}
		return 0
	}
	if err := discovery.Merge(cachePath, updates); err != nil {
		fmt.Fprintf(errW, "discover: write cache: %v\n", err)
		return 1
	}
	if !*asJSON {
		fmt.Fprintf(w, "\nwritten to %s (hooks use it before the operator session-models.json; availability only, not credit)\n", cachePath)
	}
	return 0
}

func printDiscovered(w io.Writer, harness, source string, found []discovery.Model, r discovery.Ranked) {
	fmt.Fprintf(w, "%-12s %-28s %d offered, %d usable\n", harness, source, len(found), len(r.IDs))
	if len(r.IDs) > 0 {
		fmt.Fprintf(w, "  order (least → most capable): %s\n", strings.Join(r.IDs, " < "))
	}
	if len(r.FamilyOnly) > 0 {
		var parts []string
		for _, n := range r.FamilyOnly {
			parts = append(parts, fmt.Sprintf("%s (tier of %s)", n.ID, n.CatalogID))
		}
		fmt.Fprintf(w, "  not in the catalog by id, tier inherited from its family: %s\n", strings.Join(parts, ", "))
	}
	if len(r.Superseded) > 0 {
		sort.Strings(r.Superseded)
		fmt.Fprintf(w, "  superseded by a newer model of the same family: %s\n", strings.Join(r.Superseded, ", "))
	}
	if len(r.Unranked) > 0 {
		fmt.Fprintf(w, "  no catalog tier, never a target: %s\n", strings.Join(r.Unranked, ", "))
	}
}
