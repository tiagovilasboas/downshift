// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"github.com/tiagovilasboas/downshift/internal/codeintel"
)

// dciEnableArgs is the parsed `downshift dci enable` command line.
type dciEnableArgs struct {
	mode      codeintel.Mode
	budget    int
	budgetSet bool
	dryRun    bool
}

func clampDCIBudget(b int) int {
	if b < 512 {
		return 512
	}
	if b > 16384 {
		return 16384
	}
	return b
}

func parseDCIBudget(v string) (int, error) {
	val, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("invalid budget %q: must be an integer", v)
	}
	return clampDCIBudget(val), nil
}

func parseDCIEnableArgs(args []string) (dciEnableArgs, error) {
	parsed := dciEnableArgs{mode: codeintel.ModeActive}
	modeSet := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--dry-run":
			parsed.dryRun = true
		case strings.HasPrefix(arg, "--budget="):
			budget, err := parseDCIBudget(strings.TrimPrefix(arg, "--budget="))
			if err != nil {
				return parsed, err
			}
			parsed.budget, parsed.budgetSet = budget, true
		case arg == "--budget":
			if i+1 >= len(args) {
				return parsed, fmt.Errorf("missing value for --budget")
			}
			i++
			budget, err := parseDCIBudget(args[i])
			if err != nil {
				return parsed, err
			}
			parsed.budget, parsed.budgetSet = budget, true
		case !modeSet:
			switch strings.ToLower(arg) {
			case "observe", "obs":
				parsed.mode = codeintel.ModeObserve
			case "active":
				parsed.mode = codeintel.ModeActive
			default:
				return parsed, fmt.Errorf("invalid mode %q: choose 'active' or 'observe'", arg)
			}
			modeSet = true
		default:
			return parsed, fmt.Errorf("unexpected argument %q", arg)
		}
	}
	return parsed, nil
}

func runDCIEnable(args []string, w io.Writer, stderr io.Writer) int {
	parsed, err := parseDCIEnableArgs(args)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	repoRoot := findRepoRoot()
	current, err := codeintel.LoadConfig(repoRoot)
	if err != nil {
		current = codeintel.DefaultConfig()
	}
	cfg := current
	cfg.Mode = parsed.mode
	switch {
	case parsed.budgetSet:
		cfg.MaxContextBytes = parsed.budget
	case cfg.MaxContextBytes <= 0:
		cfg.MaxContextBytes = 3500
	default:
		cfg.MaxContextBytes = clampDCIBudget(cfg.MaxContextBytes)
	}

	if parsed.dryRun {
		return printDCIEnablePlan(w, stderr, repoRoot, current.Mode, cfg)
	}
	if err := codeintel.SaveConfig(repoRoot, cfg); err != nil {
		fmt.Fprintf(stderr, "failed to enable DCI: %v\n", err)
		return 1
	}
	fmt.Fprintf(w, "Downshift Code Intelligence (DCI) enabled in mode %q (budget: %d bytes).\n", cfg.Mode, cfg.MaxContextBytes)
	return 0
}

// printDCIEnablePlan reports the repo config `dci enable` would write and the
// prompt hook it depends on. It writes nothing.
func printDCIEnablePlan(w, stderr io.Writer, repoRoot string, from codeintel.Mode, cfg codeintel.Config) int {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		fmt.Fprintf(stderr, "failed to plan DCI config: %v\n", err)
		return 1
	}
	fmt.Fprintln(w, "DCI dry run: nothing written.")
	fmt.Fprintf(w, "Would write %s (mode %s -> %s, budget %d bytes):\n", codeintel.ConfigPath(repoRoot), from, cfg.Mode, cfg.MaxContextBytes)
	fmt.Fprintln(w, string(data))
	hooks := dciPromptHooks()
	if len(hooks) == 0 {
		fmt.Fprintln(w, "No harness gets a DCI prompt hook from `downshift setup`.")
		return 0
	}
	fmt.Fprintln(w, "The config only acts where the prompt hook runs. `downshift setup` installs it for:")
	for _, h := range hooks {
		fmt.Fprintf(w, "  %s: %s -> downshift %s\n", h.harness, h.event, h.args)
	}
	fmt.Fprintln(w, "Preview that write with `downshift setup --dry-run`. The Claude Code plugin ships the same hook.")
	return 0
}

type dciPromptHook struct {
	harness, event, args string
}

// dciPromptHooks lists the DCI hooks setup declares, read from setupTargets
// so the plan cannot drift from what setup writes.
func dciPromptHooks() []dciPromptHook {
	var out []dciPromptHook
	for _, target := range setupTargets {
		for _, hook := range target.Hooks {
			if strings.HasPrefix(hook.Args, "dci hook ") {
				out = append(out, dciPromptHook{harness: target.Name, event: hook.Event, args: hook.Args})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].harness < out[j].harness })
	return out
}
