// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package benchmark

import (
	"embed"
	"fmt"
	"io"
	"time"

	"github.com/tiagovilasboas/downshift/internal/compressor"
)

//go:embed fixtures/go_test_all_ok.txt fixtures/go_test_non_ok.txt fixtures/repetitive_logs.txt fixtures/search_hits.txt
var compressorFixtures embed.FS

// ContextScenario represents one of the four benchmark comparison scenarios.
type ContextScenario string

const (
	ScenarioA ContextScenario = "Scenario A: Baseline (No Downshift, No Compression)"
	ScenarioB ContextScenario = "Scenario B: Downshift only (Model routing)"
	ScenarioC ContextScenario = "Scenario C: Native Compressor only (Context compression)"
	ScenarioD ContextScenario = "Scenario D: Downshift + Native Compressor (Routing + Compression)"
)

// ContextBenchmarkRecord represents a synthetic or captured task execution across scenarios.
type ContextBenchmarkRecord struct {
	TaskDescription string          `json:"task_description"`
	Category        string          `json:"category"` // file_listing, repo_search, test_run, long_logs, debugging, multi_file, subagent_spawn
	Scenario        ContextScenario `json:"scenario"`
	InputTokens     int64           `json:"input_tokens"`
	OutputTokens    int64           `json:"output_tokens"`
	TotalCostUSD    float64         `json:"total_cost_usd"`
	Latency         time.Duration   `json:"latency"`
	SuccessRate     float64         `json:"success_rate"` // 0.0 - 1.0
	Retries         int             `json:"retries"`
	ToolCalls       int             `json:"tool_calls"`
}

// CompareContextScenarios prints the routing scenario matrix and a measured
// compressor byte table. Routing cost and accuracy stay "not measured".
// The byte table runs the compressor on checked-in fixtures. It reports
// original bytes, reduced bytes, and byte savings. It does not estimate
// tokens or dollars.
func CompareContextScenarios(w io.Writer) error {
	fmt.Fprintln(w, "================================================================================")
	fmt.Fprintln(w, "Downshift Context Optimization")
	fmt.Fprintln(w, "Routing cost and accuracy are not measured. Do not cite 68, 35, or 80.")
	fmt.Fprintln(w, "Compressor rows below are measured bytes from checked-in fixtures.")
	fmt.Fprintln(w, "================================================================================")

	scenarios := []struct {
		Name        ContextScenario
		RoutingTier string
		Compression string
		CostFactor  string
		Accuracy    string
	}{
		{ScenarioA, "Frontier (Unrouted)", "None (Raw logs)", "not measured", "not measured"},
		{ScenarioB, "Downshift Tiered", "None (Raw logs)", "not measured", "not measured"},
		{ScenarioC, "Frontier (Unrouted)", "Native Squelch", "not measured", "not measured"},
		{ScenarioD, "Downshift Tiered", "Native Squelch", "not measured", "not measured"},
	}

	fmt.Fprintf(w, "%-46s | %-16s | %-15s | %-18s | %-12s\n", "Scenario", "Routing", "Compression", "Cost Factor", "Accuracy")
	fmt.Fprintln(w, "----------------------------------------------------------------------------------------------------------------")
	for _, s := range scenarios {
		fmt.Fprintf(w, "%-46s | %-16s | %-15s | %-18s | %-12s\n", s.Name, s.RoutingTier, s.Compression, s.CostFactor, s.Accuracy)
	}
	fmt.Fprintln(w, "----------------------------------------------------------------------------------------------------------------")
	fmt.Fprintln(w, "Measured compressor bytes (safe mode, exit 0). Unit: bytes.")
	fixtures := []struct {
		Name string
		File string
	}{
		{"go_test_ok", "fixtures/go_test_all_ok.txt"},
		{"go_test_warning", "fixtures/go_test_non_ok.txt"},
		{"repetitive_logs", "fixtures/repetitive_logs.txt"},
		{"search_hits", "fixtures/search_hits.txt"},
	}
	for _, fx := range fixtures {
		raw, err := compressorFixtures.ReadFile(fx.File)
		if err != nil {
			return fmt.Errorf("read fixture %s: %w", fx.File, err)
		}
		res := compressor.CompressExit(raw, compressor.ModeSafe, 0)
		savings := res.OriginalBytes - res.ReducedBytes
		if savings < 0 {
			savings = 0
		}
		fmt.Fprintf(w, "%s original=%d bytes reduced=%d bytes savings=%d bytes\n", fx.Name, res.OriginalBytes, res.ReducedBytes, savings)
	}
	fmt.Fprintln(w, "Key Insights:")
	fmt.Fprintln(w, "1. Model routing and compression are separate levers. This command does not price routing.")
	fmt.Fprintln(w, "2. Safe compression keeps non-ok test lines, search hits, file names, git logs, and any non-zero exit.")
	fmt.Fprintln(w, "3. Routing cost, tokens, and accuracy are not measured. Do not cite 68, 35, or 80 as evidence.")
	fmt.Fprintln(w, "4. The fixture table is bytes only. It is not a token count and not a dollar saving.")
	return nil
}
