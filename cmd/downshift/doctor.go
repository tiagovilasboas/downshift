// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/tiagovilasboas/downshift/internal/catalog"
	"github.com/tiagovilasboas/downshift/internal/contextopt"
	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/paths"
)

func runDoctor(w io.Writer, cat *catalog.Catalog) int {
	dir, src, err := paths.Resolve()
	if err != nil {
		fmt.Fprintf(w, "state_dir: error: %v\n", err)
		return 1
	}
	fmt.Fprintf(w, "version: downshift %s\n", buildVersion)
	if buildCommit != "" {
		fmt.Fprintf(w, "commit: %s\n", buildCommit)
	}
	fmt.Fprintf(w, "state_dir: %s\n", dir)
	fmt.Fprintf(w, "state_dir_source: %s\n", src)
	if src == paths.SourceLegacy {
		fmt.Fprintf(w, "migration_hint: copy %s to %s/%s when ready (or set %s)\n",
			dir, os.Getenv("HOME"), paths.DirName, paths.EnvStateDir)
	}

	catalogPath, _ := paths.CatalogPath()
	if _, err := os.Stat(catalogPath); err == nil {
		fmt.Fprintf(w, "catalog: user override at %s\n", catalogPath)
	} else {
		fmt.Fprintf(w, "catalog: embedded default (override path %s)\n", catalogPath)
	}

	eventsPath, _ := paths.EventsPath()
	if _, err := os.Stat(eventsPath); err == nil {
		fmt.Fprintf(w, "events: %s (present)\n", eventsPath)
	} else {
		fmt.Fprintf(w, "events: %s (not created yet)\n", eventsPath)
	}

	optCfg, _ := contextopt.LoadConfig()
	if optCfg.Enabled {
		fmt.Fprintf(w, "context_optimization: enabled (provider: %s)\n", optCfg.Provider)
	} else {
		fmt.Fprintf(w, "context_optimization: disabled (run `downshift context enable`)\n")
	}

	cmd := os.Getenv("DOWNSHIFT_MINILM_EMBED")
	if cmd != "" && cmd != "hash" {
		fmt.Fprintf(w, "semantic_boost: external command configured (%s, hash fallback on failure)\n", cmd)
	} else {
		fmt.Fprintf(w, "semantic_boost: in-process hash embedder (default, zero-dependency)\n")
	}

	for _, note := range modelOverrideNotes(core.AllHarnessCaps(), os.Getenv) {
		fmt.Fprintln(w, note)
	}

	if cat != nil {
		if unknown := cat.EffortUnknownModels(); len(unknown) > 0 {
			fmt.Fprintf(w, "effort_unknown: %d models without a verified efforts list (effort not constrained): %s\n",
				len(unknown), strings.Join(unknown, ", "))
		}
	}
	return 0
}

// modelOverrideNotes reports every declared model-override variable that is
// set. A harness-level model setting can take precedence over a hook's model
// rewrite, which would make a Downshift rewrite a silent no-op. The precedence
// comes from outside sources and is not verified here, so the wording stays
// conditional. Nothing about routing changes. Adding a harness's variable is a
// one-line capability change; doctor needs no edit.
func modelOverrideNotes(caps []core.NamedCaps, getenv func(string) string) []string {
	var notes []string
	for _, c := range caps {
		name := c.Caps.ModelOverrideEnv
		if name == "" {
			continue
		}
		v := strings.TrimSpace(getenv(name))
		if v == "" {
			continue
		}
		if len(v) > 64 {
			v = v[:64] + "..."
		}
		notes = append(notes, fmt.Sprintf("model_override_env: %s=%s; %s may apply it instead of a Downshift rewrite (not verified here). Check the resolved model with the post-tool-use hook.", name, v, c.Harness))
	}
	return notes
}
