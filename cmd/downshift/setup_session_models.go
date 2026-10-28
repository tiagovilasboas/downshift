// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/tiagovilasboas/downshift/internal/paths"
)

// defaultSessionModels provides the canonical relative tiers (cheap, standard, frontier)
// for each supported harness when session-models.json has no entry for it.
var defaultSessionModels = map[string][]string{
	"claude-code": {
		"claude-haiku-5-5",
		"claude-sonnet-5-5",
		"claude-opus-5-5",
	},
	"antigravity": {
		"flash_lite",
		"flash",
		"pro",
		"inherit",
	},
	"cursor": {
		"composer-2.5-fast",
		"composer-2.5",
		"claude-sonnet-5-5-high",
		"claude-opus-5-5-medium",
	},
	"codex": {
		"gpt-6-luna",
		"gpt-5.6-terra",
		"gpt-6-sol",
	},
	"kirocrew": {
		"claude-haiku-5-5",
		"claude-sonnet-5-5",
		"claude-opus-5-5",
	},
	"opencode": {
		"minimax/minimax-m3.1-flash:free",
		"deepseek/deepseek-v4.1-flash-1017",
		"big-pickle",
	},
}

// setupEnsureSessionModels discovers and seeds session-models.json for detected harnesses.
func setupEnsureSessionModels(o setupOptions, targets []setupTarget) {
	if o.DryRun || o.Undo {
		return
	}

	p, err := paths.SessionModelsPath()
	if err != nil {
		return
	}

	data := map[string]any{}
	if raw, err := os.ReadFile(p); err == nil {
		_ = json.Unmarshal(raw, &data)
	}

	changed := false
	for _, target := range targets {
		h := target.Name
		if _, exists := data[h]; !exists {
			if defs, ok := defaultSessionModels[h]; ok {
				data[h] = defs
				changed = true
			}
		}
	}

	if changed {
		_ = os.MkdirAll(filepath.Dir(p), 0700)
		if formatted, err := json.MarshalIndent(data, "", " "); err == nil {
			_ = os.WriteFile(p, formatted, 0600)
			fmt.Fprintf(o.Out, "session-models: updated %s with detected relative tiers\n", paths.DisplaySessionModels())
		}
	}
}
