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

	if note := subagentModelEnvNote(os.Getenv); note != "" {
		fmt.Fprintln(w, note)
	}

	_ = cat // reserved for future: effective harness count, shadow flag, etc.
	return 0
}

// subagentModelEnvNote reports CLAUDE_CODE_SUBAGENT_MODEL when it is set.
// A harness-level model setting can take precedence over a hook's
// updatedInput.model, which would make a Downshift rewrite a silent no-op.
// This is reported by a third-party router and not verified here, so the
// wording stays conditional. Nothing about routing changes.
func subagentModelEnvNote(getenv func(string) string) string {
	const name = "CLAUDE_CODE_SUBAGENT_MODEL"
	v := strings.TrimSpace(getenv(name))
	if v == "" {
		return ""
	}
	if len(v) > 64 {
		v = v[:64] + "..."
	}
	return fmt.Sprintf("subagent_model_env: %s=%s; Claude Code may apply it instead of a Downshift rewrite (not verified here). Check the resolved model with the post-tool-use hook.", name, v)
}
