// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/tiagovilasboas/downshift-ai/internal/compressor"
	"github.com/tiagovilasboas/downshift-ai/internal/contextopt"
	"github.com/tiagovilasboas/downshift-ai/internal/execwrap"
	"github.com/tiagovilasboas/downshift-ai/internal/sensor"
)

func runContextMetrics(ctx context.Context, reg *contextopt.Registry, w, errW io.Writer) int {
	cfg, err := contextopt.LoadConfig()
	if err != nil || !cfg.Enabled {
		fmt.Fprintln(errW, "Context optimization is not enabled. Run `downshift context enable native` first.")
		return 1
	}
	p, ok := reg.Get(cfg.Provider)
	if !ok {
		fmt.Fprintf(errW, "Active provider %q is not registered.\n", cfg.Provider)
		return 1
	}

	cwd, _ := os.Getwd()
	metrics, err := p.GetMetrics(ctx, cwd)
	if err != nil {
		fmt.Fprintf(errW, "Failed to retrieve provider metrics: %v\n", err)
	}

	st, _ := sensor.DefaultStore()
	sensorObservations := map[string]sensor.SessionObservation{}
	if st != nil {
		sensorObservations, _ = st.GetSummary()
	}

	var execMetrics *execwrap.Summary
	if path, err := execwrap.EventsPath(); err == nil {
		if s, err := execwrap.Summarize(path); err == nil && s.Commands > 0 {
			execMetrics = &s
		}
	}
	if sp, err := execwrap.SkipsPath(); err == nil {
		if skips, err := execwrap.SummarizeSkips(sp); err == nil && len(skips) > 0 {
			if execMetrics == nil {
				execMetrics = &execwrap.Summary{}
			}
			execMetrics.SkipsByReason = skips
			execMetrics.SkipsByMetachar, _ = execwrap.SummarizeSkipClasses(sp)
			execMetrics.SkipsByKind, _ = execwrap.SummarizeSkipKinds(sp)
		}
	}

	output := struct {
		ProviderMetrics    *contextopt.Metrics                  `json:"provider_metrics,omitempty"`
		SensorObservations map[string]sensor.SessionObservation `json:"context_sensor_observations,omitempty"`
		ExecMetrics        *execwrap.Summary                    `json:"exec_metrics,omitempty"`
	}{
		ProviderMetrics:    metrics,
		SensorObservations: sensorObservations,
		ExecMetrics:        execMetrics,
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(output)
	return 0
}

func runContextCompress(args []string, w, errW io.Writer) int {
	mode := compressor.ModeSafe
	exitCode := -1
	exitKnown := false
	var positional []string
	for i := 0; i < len(args); i++ {
		if args[i] == "--exit" {
			if i+1 >= len(args) {
				fmt.Fprintln(errW, "context compress: --exit requires a status from 0 to 255")
				return 2
			}
			n, err := strconv.Atoi(args[i+1])
			if err != nil || n < 0 || n > 255 {
				fmt.Fprintln(errW, "context compress: --exit requires a status from 0 to 255")
				return 2
			}
			exitCode = n
			exitKnown = true
			i++
			continue
		}
		positional = append(positional, args[i])
	}
	if len(positional) > 0 {
		switch positional[0] {
		case "observe", "--observe":
			mode = compressor.ModeObserve
		case "safe", "--safe":
			mode = compressor.ModeSafe
		case "off", "--off":
			mode = compressor.ModeOff
		default:
			fmt.Fprintf(errW, "unknown compress mode %q (use observe, safe, or off)\n", positional[0])
			return 2
		}
	}

	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintf(errW, "error reading stdin: %v\n", err)
		return 1
	}

	if mode == compressor.ModeSafe && !exitKnown {
		fmt.Fprintln(errW, "context compress: safe mode requires --exit 0. Output preserved.")
		_, _ = w.Write(data)
		return 0
	}

	res := compressor.CompressExit(data, mode, exitCode)
	if mode == compressor.ModeObserve {
		fmt.Fprintf(errW, "context observe: format=%s original=%d bytes potential_reduced=%d bytes savings=%.1f%% (%s)\n",
			res.Format, res.OriginalBytes, res.ReducedBytes, res.SavingsRatio(), res.Reason)
		_, _ = w.Write(res.Output)
		return 0
	}

	if res.Applied {
		fmt.Fprintf(errW, "context compress: %s (reduced from %d to %d bytes, %.1f%% saved)\n",
			res.Reason, res.OriginalBytes, res.ReducedBytes, res.SavingsRatio())
	}
	_, _ = w.Write(res.Output)
	return 0
}

// printExecEnabled says what compact does and the one hook entry it needs.
// It writes no harness settings: the plugin and setup carry the Bash hook.
func printExecEnabled(w io.Writer, on bool) {
	if !on {
		fmt.Fprintln(w, "Compact: off (\"exec\": false in the config, or DOWNSHIFT_EXEC=0). Remove the key and unset the variable to turn it on.")
		return
	}
	fmt.Fprintln(w, "Compact: on. Allowlisted shell commands (go test, any git command) with no shell metacharacters")
	fmt.Fprintln(w, "are rewritten to `downshift compact -- <command>`, which keeps the exit code and compacts only clean output.")
	fmt.Fprintln(w, "Your own rules still gate it: a command is not rewritten when a deny or ask rule in any settings scope")
	fmt.Fprintln(w, "matches it. No allow rule is installed or required; your permission mode decides any approval prompt.")
	fmt.Fprintln(w, "Other harnesses: not wired.")
}

// compactOn reports whether the compact wrapper is on: the environment kill
// switch and the config key both have to allow it.
func compactOn(cfg contextopt.Config) bool {
	return os.Getenv(execEnvKillSwitch) != "0" && cfg.ExecOn()
}
