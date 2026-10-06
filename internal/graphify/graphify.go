// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package graphify provides codebase-graph-aware escalation hints for the
// Downshift classifier. It detects when a task prompt references a file or
// symbol that, according to the knowledge graph, is highly connected or lives
// in a high-risk community — and returns a hint that can escalate the routing
// decision to COMPLEX without any LLM call.
//
// Integration contract:
//
//	hint := graphify.Hint(prompt, graphify.DefaultCriteria())
//	if hint.ShouldEscalate {
//	    // override the classifier result to core.Complex
//	}
//
// Zero network when no file/symbol is detected. The MCP call happens only when
// a file path or known high-risk symbol is found in the prompt.
//
// NOTE: The actual MCP/graphify call (query_graph, god_nodes, get_pr_impact)
// is intended to be injected by the caller via the GraphFetcher interface so
// that Go adapters can call the Kiro Crew graphify MCP tools. In offline mode
// (e.g. tests, CLI without Kiro), the caller injects a no-op fetcher and
// escalation falls back to text-only signals.
package graphify

import (
	"regexp"
	"strconv"
	"strings"
)

// Community names that represent high-risk domains in the Voomp codebase graph.
// Matched case-insensitively against the community label returned by the graph.
var highRiskCommunities = []string{
	"client & subscription state",
	"user & consent",
	"repository interfaces",
	"erp controllers",
	"community 27", // CreateSubaccount — confirmed high-risk by god_nodes
}

// EdgeThreshold is the minimum number of graph edges a node must have to be
// considered "highly connected" and trigger an escalation hint.
const EdgeThreshold = 20

// filePathPattern matches relative or absolute PHP/TS/JS/Go file mentions.
var filePathPattern = regexp.MustCompile(
	`(?i)\b(src/|app/|internal/|pkg/|cmd/)[\w./\-]+\.(php|ts|js|go|tsx)\b`,
)

// symbolPattern matches PascalCase symbols likely to be class/function names.
var symbolPattern = regexp.MustCompile(`\b[A-Z][A-Za-z0-9]{5,}(Service|Controller|Repository|Gateway|Handler|Command|Observer|Job|Middleware)\b`)

// Criteria controls when the graphify adapter triggers and what it escalates.
type Criteria struct {
	// EdgeThreshold: escalate when a mentioned node has >= this many edges.
	EdgeThreshold int
	// HighRiskCommunities: escalate when the node's community matches any entry.
	HighRiskCommunities []string
}

// DefaultCriteria returns the standard escalation criteria for the Voomp
// codebase — tuned against the god_nodes and community data observed in the
// active graph (4934 nodes, 183 communities, 2026-09-30).
func DefaultCriteria() Criteria {
	return Criteria{
		EdgeThreshold:       EdgeThreshold,
		HighRiskCommunities: highRiskCommunities,
	}
}

// NodeInfo is the minimal graph data needed to make an escalation decision.
// Callers populate this from the graphify MCP tool results (get_node,
// query_graph, god_nodes).
type NodeInfo struct {
	Label     string // symbol or file name
	Community string // community label from the graph
	Edges     int    // number of edges (degree)
	Found     bool   // false when the graph returned no data for this node
}

// GraphFetcher is the interface adapters implement to call the actual graphify
// MCP tool. In production the Kiro Crew adapter calls query_graph/get_node;
// in tests a stub returns deterministic data.
type GraphFetcher interface {
	// FetchNode retrieves graph data for a file path or symbol label.
	// Returns NodeInfo with Found=false when the graph has no data for it.
	FetchNode(label string) NodeInfo
}

// noopFetcher is the offline fallback — never escalates on graph data.
type noopFetcher struct{}

func (noopFetcher) FetchNode(_ string) NodeInfo { return NodeInfo{Found: false} }

// EscalationHint is the result of a graphify analysis pass.
type EscalationHint struct {
	// ShouldEscalate is true when the graph analysis found a reason to escalate
	// the task to COMPLEX — the caller should override the classifier result.
	ShouldEscalate bool
	// Reason is a human-readable explanation for the escalation (empty when
	// ShouldEscalate is false).
	Reason string
	// TriggeringNode is the first node that caused the escalation (empty when
	// ShouldEscalate is false).
	TriggeringNode string
}

// Hint analyses a task prompt and returns an escalation hint.
//
// When fetcher is nil, a no-op fetcher is used (offline mode — no escalation
// from graph data, but text-based detection still runs for community matches).
func Hint(prompt string, criteria Criteria, fetcher GraphFetcher) EscalationHint {
	if fetcher == nil {
		fetcher = noopFetcher{}
	}

	// 1. Extract candidate labels from the prompt.
	candidates := extractCandidates(prompt)
	if len(candidates) == 0 {
		return EscalationHint{}
	}

	// 2. Query graph for each candidate; stop at the first escalation trigger.
	for _, label := range candidates {
		info := fetcher.FetchNode(label)
		if !info.Found {
			continue
		}

		// Edge-count escalation.
		if info.Edges >= criteria.EdgeThreshold {
			return EscalationHint{
				ShouldEscalate: true,
				Reason: "node '" + info.Label + "' has " + strconv.Itoa(info.Edges) +
					" graph edges (threshold: " + strconv.Itoa(criteria.EdgeThreshold) + ")" +
					" — high blast radius",
				TriggeringNode: info.Label,
			}
		}

		// High-risk community escalation.
		communityLower := strings.ToLower(info.Community)
		for _, risky := range criteria.HighRiskCommunities {
			if strings.Contains(communityLower, strings.ToLower(risky)) {
				return EscalationHint{
					ShouldEscalate: true,
					Reason: "node '" + info.Label + "' lives in community '" + info.Community +
						"' — payment/KYC/subscription critical path",
					TriggeringNode: info.Label,
				}
			}
		}
	}

	return EscalationHint{}
}

// extractCandidates returns file paths and high-signal symbols from a prompt.
func extractCandidates(prompt string) []string {
	var out []string
	seen := make(map[string]bool)

	for _, m := range filePathPattern.FindAllString(prompt, 5) {
		if !seen[m] {
			seen[m] = true
			out = append(out, m)
		}
	}
	for _, m := range symbolPattern.FindAllString(prompt, 5) {
		if !seen[m] {
			seen[m] = true
			out = append(out, m)
		}
	}
	return out
}
