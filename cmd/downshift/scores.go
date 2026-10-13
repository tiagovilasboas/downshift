// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"sort"
	"time"

	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/scoreindex"
)

// scoresSources and scoresClient let tests point the refresh at httptest
// servers. Nil means the real public sources.
var (
	scoresSources func() []scoreindex.Source
	scoresClient  *http.Client
)

// scoreSessionIDs resolves the enabled session models of every registered
// harness the way the hooks do (core.ResolveSessionForID without a payload
// list). A harness whose list is unknown maps to an empty slice.
func scoreSessionIDs() map[string][]string {
	out := make(map[string][]string, len(harnessRegistry))
	names := make([]string, 0, len(harnessRegistry))
	for name := range harnessRegistry {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		s := core.ResolveSessionForID(name, "")
		if s.Known {
			out[name] = append([]string(nil), s.IDs...)
		} else {
			out[name] = nil
		}
	}
	return out
}

// runModelsScores implements `downshift models scores [--refresh] [--status]
// [--quiet] [--json]`. Without flags it prints the status.
func runModelsScores(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("models scores", flag.ContinueOnError)
	fs.SetOutput(stderr)
	refresh := fs.Bool("refresh", false, "fetch the public sources now and rewrite scores.json")
	status := fs.Bool("status", false, "print cache age, selected source and per-harness coverage")
	quiet := fs.Bool("quiet", false, "no output on success (used by the background refresh)")
	asJSON := fs.Bool("json", false, "print the status as JSON")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if !*refresh {
		*status = true
	}
	sessions := scoreSessionIDs()

	if *refresh {
		opts := scoreindex.Options{Sessions: sessions, Version: buildVersion, Client: scoresClient}
		if scoresSources != nil {
			opts.Sources = scoresSources()
		}
		ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
		defer cancel()
		rep, err := scoreindex.Refresh(ctx, opts)
		switch {
		case errors.Is(err, scoreindex.ErrDisabled), errors.Is(err, scoreindex.ErrLocked):
			if !*quiet {
				fmt.Fprintf(stdout, "scores: %v\n", err)
			}
			return 0
		case err != nil:
			if rep != nil && !*quiet {
				for _, r := range rep.Results {
					fmt.Fprintf(stderr, "  %-14s %s\n", r.Name, r.Status)
				}
			}
			fmt.Fprintf(stderr, "scores: refresh failed: %v\n", err)
			return 1
		}
		if !*quiet && !*asJSON {
			for _, r := range rep.Results {
				fmt.Fprintf(stdout, "  %-14s rows %-3d %s\n", r.Name, r.Rows, r.Status)
			}
			fmt.Fprintf(stdout, "scores: updated, selected source %s\n", rep.File.SelectedSource)
		}
	}

	if *quiet || (!*status && !*asJSON) {
		return 0
	}
	st, err := scoreindex.BuildStatus("", sessions, time.Now())
	if err != nil {
		fmt.Fprintf(stderr, "scores: %v\n", err)
		return 1
	}
	if *asJSON {
		if err := st.WriteJSON(stdout); err != nil {
			fmt.Fprintf(stderr, "scores: %v\n", err)
			return 1
		}
		return 0
	}
	st.WriteText(stdout)
	return 0
}
