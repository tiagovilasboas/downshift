// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/models"
)

// runModelsAvailable prints the models a spawn can use right now on one
// harness and where each came from. It only reads local state.
func runModelsAvailable(catalog models.CatalogReader, args []string, w, errW io.Writer) int {
	md, ok := catalog.(models.Metadata)
	if !ok {
		fmt.Fprintln(errW, "available: catalog does not expose model metadata")
		return 1
	}
	fs := flag.NewFlagSet("models available", flag.ContinueOnError)
	fs.SetOutput(errW)
	harness := fs.String("harness", "", "harness id")
	asJSON := fs.Bool("json", false, "print the result as JSON")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	listing, err := models.Resolve(strings.TrimSpace(*harness), md, models.Options{})
	if errors.Is(err, models.ErrUnknownHarness) {
		fmt.Fprintf(errW, "available: unknown harness %q; valid ids: %s\n", *harness, strings.Join(capsHarnessIDs(), ", "))
		return 2
	}
	if err != nil {
		fmt.Fprintf(errW, "available: %v\n", err)
		return 1
	}
	if *asJSON {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		if enc.Encode(listing) != nil {
			return 1
		}
		return 0
	}
	writeAvailableText(w, listing)
	return 0
}

func capsHarnessIDs() []string {
	var ids []string
	for _, n := range core.AllHarnessCaps() {
		ids = append(ids, n.Harness)
	}
	return ids
}

func writeAvailableText(w io.Writer, l models.Listing) {
	if len(l.Models) == 0 {
		fmt.Fprintf(w, "no detected list for %s; run downshift models discover --harness %s\n", l.Harness, l.Harness)
		return
	}
	switch {
	case !l.CacheWrittenAt.IsZero():
		fmt.Fprintf(w, "%s: discovery cache written %s\n", l.Harness, l.CacheWrittenAt.UTC().Format(time.RFC3339))
	case l.Fallback:
		fmt.Fprintf(w, "%s: catalog stand-in only (the harness sends no list); run downshift models discover --harness %s\n", l.Harness, l.Harness)
	default:
		fmt.Fprintf(w, "%s: detected models\n", l.Harness)
	}
	for _, m := range l.Models {
		line := fmt.Sprintf("  %-32s %-9s %s", m.ID, m.Tier, m.Source)
		if len(m.Efforts) > 0 {
			line += "  efforts=" + strings.Join(m.Efforts, ",")
		}
		if m.Untiered {
			line += "  [untiered: never a hint or rule target]"
		}
		fmt.Fprintln(w, line)
	}
}
