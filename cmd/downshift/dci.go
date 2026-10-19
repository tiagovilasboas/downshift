// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/tiagovilasboas/downshift/internal/codeintel"
)

func runDCI(args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 {
		printDCIHelp(stdout)
		return 0
	}

	switch args[0] {
	case "status":
		return runDCIStatus(stdout, stderr)
	case "enable":
		return runDCIEnable(args[1:], stdout, stderr)
	case "disable":
		return runDCIDisable(stdout, stderr)
	case "doctor":
		return runDCIDoctor(stdout, stderr)
	case "inspect":
		return runDCIInspect(args[1:], stdout, stderr)
	case "impact":
		return runDCIImpact(args[1:], stdout, stderr)
	case "benchmark":
		return runDCIBenchmark(args[1:], stdout, stderr)
	case "hook":
		if len(args) < 2 {
			fmt.Fprintln(stderr, "usage: downshift dci hook <harness>")
			return 2
		}
		return runDCIHook(args[1], stdin, stdout, stderr)
	case "-h", "--help", "help":
		printDCIHelp(stdout)
		return 0
	default:
		fmt.Fprintf(stderr, "unknown dci command: %s\n\n", args[0])
		printDCIHelp(stderr)
		return 2
	}
}

func printDCIHelp(w io.Writer) {
	fmt.Fprintln(w, `Usage: downshift dci <command> [arguments]

Commands:
  status                              Show current DCI operational mode, index cache, and statistics
  enable [active|obs] [--budget=N]    Enable DCI (active: injects context; observe: telemetry only; budget: 512..16384 bytes, default 3500)
  disable                             Disable DCI (mode off)
  doctor                              Diagnose DCI health, file limits, permissions, and budgets
  inspect [--json] <prompt>           Simulate retrieval and context generation for a query
  impact [--json] <symbol_or_file>    Inspect impact blast radius (callers and file dependencies)
  benchmark                           Run automated latency, relevance, and budget benchmarks
  hook <harness>                      Run hook handler on stdin for harness (claude-code, cursor, opencode, codex, generic)`)
}

func findRepoRoot() string {
	cwd, err := os.Getwd()
	if err != nil {
		return "."
	}
	dir := cwd
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return cwd
}

func runDCIStatus(w io.Writer, stderr io.Writer) int {
	repoRoot := findRepoRoot()
	cfg, err := codeintel.LoadConfig(repoRoot)
	if err != nil {
		fmt.Fprintf(stderr, "error loading config: %v\n", err)
		return 1
	}

	cachePath := filepath.Join(repoRoot, codeintel.DefaultCacheRelativePath)
	idx, err := codeintel.LoadCache(cachePath)

	fmt.Fprintln(w, "Downshift Code Intelligence (DCI) Status:")
	fmt.Fprintf(w, "  Workspace Root: %s\n", repoRoot)
	fmt.Fprintf(w, "  Operational Mode: %s\n", cfg.Mode)
	fmt.Fprintf(w, "  Config File:    %s\n", codeintel.ConfigPath(repoRoot))
	fmt.Fprintf(w, "  Cache File:     %s\n", cachePath)

	if err != nil || idx == nil {
		fmt.Fprintln(w, "  Index State:    Not built / Cache missing")
	} else {
		fmt.Fprintln(w, "  Index State:    Ready")
		fmt.Fprintf(w, "  Indexed Files:  %d\n", len(idx.Documents))
		fmt.Fprintf(w, "  Unique Terms:   %d\n", len(idx.Postings))
		fmt.Fprintf(w, "  Total Tokens:   %d\n", idx.TotalTokens)
		fmt.Fprintf(w, "  Avg Doc Length: %.1f tokens\n", idx.AvgDocLen)
		fmt.Fprintf(w, "  Last Updated:   %s\n", idx.LastUpdated.Format(time.RFC3339))
	}

	fmt.Fprintf(w, "  Context Budget: max %d bytes UTF-8\n", cfg.MaxContextBytes)
	fmt.Fprintf(w, "  File Limit:     max %d files (max %d KiB/file)\n", cfg.MaxFiles, cfg.MaxFileSizeBytes/1024)
	return 0
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

func runDCIEnable(args []string, w io.Writer, stderr io.Writer) int {
	repoRoot := findRepoRoot()
	mode := codeintel.ModeActive
	modeSet := false
	budget := 0
	budgetSet := false

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "--budget=") {
			valStr := strings.TrimPrefix(arg, "--budget=")
			val, err := strconv.Atoi(valStr)
			if err != nil {
				fmt.Fprintf(stderr, "invalid budget %q: must be an integer\n", valStr)
				return 2
			}
			budget = clampDCIBudget(val)
			budgetSet = true
		} else if arg == "--budget" {
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "missing value for --budget")
				return 2
			}
			i++
			val, err := strconv.Atoi(args[i])
			if err != nil {
				fmt.Fprintf(stderr, "invalid budget %q: must be an integer\n", args[i])
				return 2
			}
			budget = clampDCIBudget(val)
			budgetSet = true
		} else if !modeSet {
			m := strings.ToLower(arg)
			if m == "observe" || m == "obs" {
				mode = codeintel.ModeObserve
				modeSet = true
			} else if m == "active" {
				mode = codeintel.ModeActive
				modeSet = true
			} else {
				fmt.Fprintf(stderr, "invalid mode %q: choose 'active' or 'observe'\n", arg)
				return 2
			}
		} else {
			fmt.Fprintf(stderr, "unexpected argument %q\n", arg)
			return 2
		}
	}

	cfg, err := codeintel.LoadConfig(repoRoot)
	if err != nil {
		cfg = codeintel.DefaultConfig()
	}
	cfg.Mode = mode
	if cfg.MaxContextBytes <= 0 {
		cfg.MaxContextBytes = 3500
	}
	if budgetSet {
		cfg.MaxContextBytes = budget
	} else {
		cfg.MaxContextBytes = clampDCIBudget(cfg.MaxContextBytes)
	}

	if err := codeintel.SaveConfig(repoRoot, cfg); err != nil {
		fmt.Fprintf(stderr, "failed to enable DCI: %v\n", err)
		return 1
	}

	fmt.Fprintf(w, "Downshift Code Intelligence (DCI) enabled in mode %q (budget: %d bytes).\n", mode, cfg.MaxContextBytes)
	return 0
}

func runDCIDisable(w io.Writer, stderr io.Writer) int {
	repoRoot := findRepoRoot()
	if err := codeintel.SetMode(repoRoot, codeintel.ModeOff); err != nil {
		fmt.Fprintf(stderr, "failed to disable DCI: %v\n", err)
		return 1
	}

	fmt.Fprintln(w, "Downshift Code Intelligence (DCI) disabled (mode off).")
	return 0
}

func runDCIDoctor(w io.Writer, stderr io.Writer) int {
	repoRoot := findRepoRoot()
	fmt.Fprintln(w, "Checking Downshift Code Intelligence (DCI) Health:")

	allPassed := true

	// 1. Repo root
	fmt.Fprintf(w, "  [PASS] Repository Root: %s\n", repoRoot)

	// 2. Config check
	cfg, err := codeintel.LoadConfig(repoRoot)
	if err != nil {
		fmt.Fprintf(w, "  [FAIL] Config: %v\n", err)
		allPassed = false
	} else {
		fmt.Fprintf(w, "  [PASS] Config Mode: %s (Context Budget: %d bytes)\n", cfg.Mode, cfg.MaxContextBytes)
	}

	// 3. File discovery check
	ctx, cancel := context.WithTimeout(context.Background(), 2500*time.Millisecond)
	defer cancel()

	discStart := time.Now()
	discovered, err := codeintel.DiscoverFiles(ctx, repoRoot, cfg)
	discDur := time.Since(discStart)

	if err != nil {
		fmt.Fprintf(w, "  [FAIL] Discovery: %v\n", err)
		allPassed = false
	} else {
		count := len(discovered)
		if count >= cfg.MaxFiles {
			fmt.Fprintf(w, "  [WARN] Discovery: found %d files (reached limit %d) in %v\n", count, cfg.MaxFiles, discDur)
		} else {
			fmt.Fprintf(w, "  [PASS] Discovery: %d files discovered in %v\n", count, discDur)
		}
	}

	// 4. Cache & Permissions
	cachePath := filepath.Join(repoRoot, codeintel.DefaultCacheRelativePath)
	if info, err := os.Stat(cachePath); err == nil {
		mode := info.Mode().Perm()
		if mode&0077 != 0 {
			fmt.Fprintf(w, "  [WARN] Cache File Permissions: %04o (expected 0600)\n", mode)
		} else {
			fmt.Fprintf(w, "  [PASS] Cache File Permissions: %04o\n", mode)
		}
	} else {
		fmt.Fprintln(w, "  [INFO] Cache File: not created yet (will be initialized on first run)")
	}

	// 5. Latency budget check
	benchService := codeintel.NewService(repoRoot, cfg)
	indexStart := time.Now()
	if err := benchService.EnsureIndex(ctx); err != nil {
		fmt.Fprintf(w, "  [FAIL] Index Build: %v\n", err)
		allPassed = false
	} else {
		indexDur := time.Since(indexStart)
		if indexDur > 2500*time.Millisecond {
			fmt.Fprintf(w, "  [WARN] Cold Index Latency: %v exceeds 2500ms budget\n", indexDur)
		} else {
			fmt.Fprintf(w, "  [PASS] Cold Index Latency: %v (budget <= 2500ms)\n", indexDur)
		}
	}

	if allPassed {
		fmt.Fprintln(w, "\nDCI Doctor: All critical checks passed.")
		return 0
	}
	fmt.Fprintln(w, "\nDCI Doctor: Issues detected above.")
	return 1
}

type dciInspectJSONOutput struct {
	PromptTokens     []string `json:"prompt_tokens"`
	Abstained        bool     `json:"abstained"`
	AbstentionReason string   `json:"abstention_reason"`
	CandidatesCount  int      `json:"candidates_count"`
	SelectedFiles    []string `json:"selected_files"`
	SymbolsCount     int      `json:"symbols_count"`
	SnippetsCount    int      `json:"snippets_count"`
	OutputBytes      int      `json:"output_bytes"`
	FormattedText    string   `json:"formatted_text"`
	ImpactRadius     []string `json:"impact_radius,omitempty"`
	LatencyMs        int64    `json:"latency_ms"`
}

func runDCIInspect(args []string, w io.Writer, stderr io.Writer) int {
	jsonOutput := false
	var promptParts []string
	for _, arg := range args {
		if arg == "--json" {
			jsonOutput = true
		} else {
			promptParts = append(promptParts, arg)
		}
	}

	if len(promptParts) == 0 {
		fmt.Fprintln(stderr, "usage: downshift dci inspect [--json] <prompt>")
		return 2
	}

	prompt := strings.Join(promptParts, " ")
	repoRoot := findRepoRoot()
	cfg, err := codeintel.LoadConfig(repoRoot)
	if err != nil {
		cfg = codeintel.DefaultConfig()
	}
	// Force active mode for inspection simulation
	cfg.Mode = codeintel.ModeActive

	ctx, cancel := context.WithTimeout(context.Background(), 3000*time.Millisecond)
	defer cancel()

	service := codeintel.NewService(repoRoot, cfg)

	queryTokens := codeintel.Tokenize(prompt)
	if queryTokens == nil {
		queryTokens = []string{}
	}

	pack, telemetry := service.ProcessPrompt(ctx, prompt)

	selectedFiles := []string{}
	symbolsCount := 0
	snippetsCount := 0
	outputBytes := 0
	formattedText := ""

	if pack != nil {
		if pack.SelectedFiles != nil {
			selectedFiles = pack.SelectedFiles
		}
		symbolsCount = pack.SymbolsCount
		snippetsCount = pack.SnippetsCount
		outputBytes = pack.OutputBytes
		formattedText = pack.FormattedText
	}

	abstained := telemetry.AbstentionReason != ""
	abstentionReason := telemetry.AbstentionReason
	if !abstained && telemetry.ErrorCode != "" {
		abstained = true
		abstentionReason = telemetry.ErrorCode
	}

	var impactRadius []string
	if pack != nil && pack.ImpactRadius != nil {
		impactRadius = pack.ImpactRadius
	}

	if jsonOutput {
		out := dciInspectJSONOutput{
			PromptTokens:     queryTokens,
			Abstained:        abstained,
			AbstentionReason: abstentionReason,
			CandidatesCount:  telemetry.CandidateCount,
			SelectedFiles:    selectedFiles,
			SymbolsCount:     symbolsCount,
			SnippetsCount:    snippetsCount,
			OutputBytes:      outputBytes,
			FormattedText:    formattedText,
			ImpactRadius:     impactRadius,
			LatencyMs:        telemetry.TotalDurationMs,
		}
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		if err := enc.Encode(out); err != nil {
			fmt.Fprintf(stderr, "failed to encode json: %v\n", err)
			return 1
		}
		return 0
	}

	fmt.Fprintln(w, "=== DCI Query Inspection ===")
	fmt.Fprintf(w, "Prompt: %q\n", prompt)
	fmt.Fprintf(w, "Query Tokens: %v\n\n", queryTokens)

	fmt.Fprintln(w, "--- Observed Telemetry ---")
	fmt.Fprintf(w, "Indexed Files:         %d\n", telemetry.IndexedFiles)
	fmt.Fprintf(w, "Candidate Count:       %d\n", telemetry.CandidateCount)
	fmt.Fprintf(w, "Selected Files:        %d\n", telemetry.SelectedFiles)
	fmt.Fprintf(w, "Selected Symbols:      %d\n", telemetry.SelectedSymbols)
	fmt.Fprintf(w, "Input Bytes:           %d\n", telemetry.InputBytes)
	fmt.Fprintf(w, "Output Bytes:          %d\n", telemetry.OutputBytes)
	fmt.Fprintf(w, "Index Duration:        %d ms\n", telemetry.IndexDurationMs)
	fmt.Fprintf(w, "Retrieval Duration:    %d ms\n", telemetry.RetrievalDurationMs)
	fmt.Fprintf(w, "Pack Duration:         %d ms\n", telemetry.PackDurationMs)
	fmt.Fprintf(w, "Total Duration:        %d ms\n", telemetry.TotalDurationMs)
	fmt.Fprintf(w, "Injection Applied:     %v\n", telemetry.InjectionApplied)

	if len(impactRadius) > 0 {
		fmt.Fprintf(w, "Impact Radius:         %s\n", strings.Join(impactRadius, ", "))
	}

	if telemetry.AbstentionReason != "" {
		fmt.Fprintf(w, "Abstention Reason:     %s\n", telemetry.AbstentionReason)
	}
	if telemetry.ErrorCode != "" {
		fmt.Fprintf(w, "Error Code:            %s\n", telemetry.ErrorCode)
	}

	if pack != nil && pack.OutputBytes > 0 {
		fmt.Fprintln(w, "\n--- Formatted Context Pack ---")
		fmt.Fprintln(w, pack.FormattedText)
	} else {
		fmt.Fprintln(w, "\n(No context injected — prompt abstained or no relevance)")
	}

	return 0
}

type dciImpactJSONOutput struct {
	Target       string   `json:"target"`
	IsFile       bool     `json:"is_file"`
	ImpactRadius []string `json:"impact_radius"`
	CallersCount int      `json:"callers_count"`
}

func runDCIImpact(args []string, w io.Writer, stderr io.Writer) int {
	jsonOutput := false
	var targetParts []string
	for _, arg := range args {
		if arg == "--json" {
			jsonOutput = true
		} else {
			targetParts = append(targetParts, arg)
		}
	}

	if len(targetParts) == 0 {
		fmt.Fprintln(stderr, "usage: downshift dci impact [--json] <symbol_or_file>")
		return 2
	}

	target := strings.Join(targetParts, " ")
	repoRoot := findRepoRoot()
	cfg, err := codeintel.LoadConfig(repoRoot)
	if err != nil {
		cfg = codeintel.DefaultConfig()
	}
	cfg.Mode = codeintel.ModeActive

	ctx, cancel := context.WithTimeout(context.Background(), 5000*time.Millisecond)
	defer cancel()

	service := codeintel.NewService(repoRoot, cfg)
	if err := service.EnsureIndex(ctx); err != nil {
		fmt.Fprintf(stderr, "failed to index repository: %v\n", err)
		return 1
	}

	idx := service.GetIndex()
	if idx == nil {
		fmt.Fprintln(stderr, "index not available")
		return 1
	}

	normTarget := filepath.ToSlash(target)
	isFile := false
	var callers []string
	seen := make(map[string]bool)

	// Check if target is a file in the index
	var targetDoc *codeintel.Document
	for path, doc := range idx.Documents {
		if path == normTarget || strings.HasSuffix(path, "/"+normTarget) || strings.EqualFold(path, normTarget) {
			isFile = true
			targetDoc = doc
			break
		}
	}

	if isFile && targetDoc != nil {
		// Target is a file: find callers of all symbols defined in this file, plus files importing it
		for _, sym := range targetDoc.Symbols {
			for _, clr := range idx.GetCallers(sym.Name) {
				if !seen[clr] {
					seen[clr] = true
					callers = append(callers, clr)
				}
			}
		}

		// Also check documents that import this file's path or package name
		baseWithoutExt := strings.TrimSuffix(filepath.Base(targetDoc.RelativePath), filepath.Ext(targetDoc.RelativePath))
		for path, doc := range idx.Documents {
			if path == targetDoc.RelativePath {
				continue
			}
			for _, imp := range doc.Imports {
				if imp == targetDoc.RelativePath || strings.HasSuffix(imp, "/"+baseWithoutExt) || imp == baseWithoutExt {
					rep := path
					if !seen[rep] {
						seen[rep] = true
						callers = append(callers, rep)
					}
				}
			}
		}
	} else {
		// Target is a symbol name
		for _, clr := range idx.GetCallers(target) {
			if !seen[clr] {
				seen[clr] = true
				callers = append(callers, clr)
			}
		}
	}

	sort.Strings(callers)
	if callers == nil {
		callers = []string{}
	}

	if jsonOutput {
		out := dciImpactJSONOutput{
			Target:       target,
			IsFile:       isFile,
			ImpactRadius: callers,
			CallersCount: len(callers),
		}
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		if err := enc.Encode(out); err != nil {
			fmt.Fprintf(stderr, "failed to encode json: %v\n", err)
			return 1
		}
		return 0
	}

	fmt.Fprintln(w, "=== DCI Impact Blast Radius ===")
	fmt.Fprintf(w, "Target:        %s\n", target)
	fmt.Fprintf(w, "Target Type:   %s\n", map[bool]string{true: "File", false: "Symbol"}[isFile])
	fmt.Fprintf(w, "Callers/Deps:  %d\n\n", len(callers))

	if len(callers) == 0 {
		fmt.Fprintln(w, "No callers or dependent files found in index.")
		return 0
	}

	for _, c := range callers {
		fmt.Fprintf(w, "  - %s\n", c)
	}

	return 0
}

func runDCIBenchmark(args []string, w io.Writer, stderr io.Writer) int {
	repoRoot := findRepoRoot()
	cfg := codeintel.DefaultConfig()
	cfg.Mode = codeintel.ModeActive

	fmt.Fprintln(w, "=== Running Downshift Code Intelligence (DCI) Benchmarks ===")

	// 1. Cold Indexing Benchmark
	ctx, cancel := context.WithTimeout(context.Background(), 5000*time.Millisecond)
	defer cancel()

	service := codeintel.NewService(repoRoot, cfg)
	coldStart := time.Now()
	if err := service.EnsureIndex(ctx); err != nil {
		fmt.Fprintf(stderr, "Cold indexing failed: %v\n", err)
		return 1
	}
	coldDuration := time.Since(coldStart)
	coldPass := coldDuration <= 2500*time.Millisecond
	fmt.Fprintf(w, "Cold Index Latency:    %v (Budget <= 2500ms) -> [%s]\n", coldDuration, passStr(coldPass))

	// 2. Warm Indexing Benchmark
	warmStart := time.Now()
	if err := service.EnsureIndex(ctx); err != nil {
		fmt.Fprintf(stderr, "Warm indexing failed: %v\n", err)
		return 1
	}
	warmDuration := time.Since(warmStart)
	warmPass := warmDuration <= 300*time.Millisecond
	fmt.Fprintf(w, "Warm Sync Latency:     %v (Budget <= 300ms)  -> [%s]\n", warmDuration, passStr(warmPass))

	// 3. Retrieval Latency & Abstention Benchmark
	testCases := []struct {
		name         string
		prompt       string
		wantAbstain  bool
		minOutputLen int
	}{
		{"Greeting Hi", "Hi there!", true, 0},
		{"Greeting Bom Dia", "Bom dia! Tudo bem?", true, 0},
		{"Chatter Thanks", "Thanks for your help!", true, 0},
		{"Code Query Auth", "How does token authentication work?", false, 50},
		{"Code Query Service", "Process prompt pipeline execution", false, 50},
	}

	var warmLatencies []time.Duration
	allCasesPassed := coldPass && warmPass

	fmt.Fprintln(w, "\nTest Queries:")
	for _, tc := range testCases {
		qStart := time.Now()
		pack, telemetry := service.ProcessPrompt(ctx, tc.prompt)
		qDur := time.Since(qStart)
		warmLatencies = append(warmLatencies, qDur)

		passed := true
		if tc.wantAbstain && telemetry.InjectionApplied {
			passed = false
		}
		if !tc.wantAbstain && (!telemetry.InjectionApplied || pack == nil || pack.OutputBytes < tc.minOutputLen) {
			passed = false
		}
		if pack != nil && pack.OutputBytes > cfg.MaxContextBytes {
			passed = false
		}

		if !passed {
			allCasesPassed = false
		}

		status := "PASS"
		if !passed {
			status = "FAIL"
		}
		fmt.Fprintf(w, "  %-22s: latency=%-6v applied=%-5v bytes=%-4d -> [%s]\n",
			tc.name, qDur, telemetry.InjectionApplied, telemetry.OutputBytes, status)
	}

	// 4. Calculate Warm Path p95 Latency
	sort.Slice(warmLatencies, func(i, j int) bool {
		return warmLatencies[i] < warmLatencies[j]
	})
	p95Idx := int(float64(len(warmLatencies)) * 0.95)
	if p95Idx >= len(warmLatencies) {
		p95Idx = len(warmLatencies) - 1
	}
	p95Duration := warmLatencies[p95Idx]
	p95Pass := p95Duration <= 300*time.Millisecond

	fmt.Fprintf(w, "\nWarm Path p95 Latency: %v (Budget <= 300ms) -> [%s]\n", p95Duration, passStr(p95Pass))

	if allCasesPassed && p95Pass {
		fmt.Fprintln(w, "\nBenchmark Result: ALL TESTS PASSED within epistemic budgets.")
		return 0
	}
	fmt.Fprintln(w, "\nBenchmark Result: ONE OR MORE TESTS EXCEEDED BUDGET OR FAILED CRITERIA.")
	return 1
}

func passStr(pass bool) string {
	if pass {
		return "PASS"
	}
	return "FAIL"
}
