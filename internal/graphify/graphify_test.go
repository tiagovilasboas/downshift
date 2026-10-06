// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package graphify_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/tiagovilasboas/harness-downshift/internal/graphify"
	"github.com/tiagovilasboas/harness-downshift/internal/hookctx"
)

// stubFetcher returns pre-configured NodeInfo per label for deterministic tests.
type stubFetcher struct {
	nodes map[string]graphify.NodeInfo
}

func (s *stubFetcher) FetchNode(label string) graphify.NodeInfo {
	n, ok := s.nodes[label]
	if !ok {
		return graphify.NodeInfo{Found: false}
	}
	return n
}

func stub(nodes map[string]graphify.NodeInfo) graphify.GraphFetcher {
	return &stubFetcher{nodes: nodes}
}

func TestHint_NoFileInPrompt(t *testing.T) {
	h := graphify.Hint("rename userId to userID", graphify.DefaultCriteria(), nil)
	if h.ShouldEscalate {
		t.Errorf("expected no escalation for trivial prompt, got reason: %q", h.Reason)
	}
}

func TestHint_FileNotInGraph(t *testing.T) {
	// File detected but graph returns nothing for it.
	h := graphify.Hint(
		"fix the bug in src/app/Services/FooService.php",
		graphify.DefaultCriteria(),
		stub(nil),
	)
	if h.ShouldEscalate {
		t.Errorf("expected no escalation when graph has no data, got: %q", h.Reason)
	}
}

func TestHint_HighEdgeCount_Escalates(t *testing.T) {
	fetcher := stub(map[string]graphify.NodeInfo{
		"src/app/Http/Controllers/Controller.php": {
			Label:     "Controller",
			Community: "Community 69",
			Edges:     186, // god node — Controller has 186 edges in the real graph
			Found:     true,
		},
	})

	h := graphify.Hint(
		"refactor src/app/Http/Controllers/Controller.php to split responsibilities",
		graphify.DefaultCriteria(),
		fetcher,
	)
	if !h.ShouldEscalate {
		t.Error("expected escalation for high-edge-count node, got none")
	}
	if h.TriggeringNode == "" {
		t.Error("expected TriggeringNode to be set")
	}
}

func TestHint_HighRiskCommunity_Escalates(t *testing.T) {
	fetcher := stub(map[string]graphify.NodeInfo{
		"src/app/Services/Payment/CreateSubscriptionStructureService.php": {
			Label:     "CreateSubscriptionStructureService",
			Community: "Client & Subscription State",
			Edges:     5, // low edges but in critical community
			Found:     true,
		},
	})

	h := graphify.Hint(
		"update src/app/Services/Payment/CreateSubscriptionStructureService.php",
		graphify.DefaultCriteria(),
		fetcher,
	)
	if !h.ShouldEscalate {
		t.Errorf("expected escalation for high-risk community, got none. reason=%q", h.Reason)
	}
}

func TestHint_SymbolPattern_Detected(t *testing.T) {
	fetcher := stub(map[string]graphify.NodeInfo{
		"KycWebhooksController": {
			Label:     "KycWebhooksController",
			Community: "User & Consent",
			Edges:     12,
			Found:     true,
		},
	})

	// Symbol detected even without a file path in prompt.
	h := graphify.Hint(
		"add retry logic to KycWebhooksController",
		graphify.DefaultCriteria(),
		fetcher,
	)
	if !h.ShouldEscalate {
		t.Errorf("expected escalation for KycWebhooksController (User & Consent community), got none. reason=%q", h.Reason)
	}
}

func TestHint_LowEdge_SafeCommunity_NoEscalation(t *testing.T) {
	fetcher := stub(map[string]graphify.NodeInfo{
		"src/app/Services/Util/DateHelperService.php": {
			Label:     "DateHelperService",
			Community: "Community 99",
			Edges:     3,
			Found:     true,
		},
	})

	h := graphify.Hint(
		"fix timezone bug in src/app/Services/Util/DateHelperService.php",
		graphify.DefaultCriteria(),
		fetcher,
	)
	if h.ShouldEscalate {
		t.Errorf("expected no escalation for low-risk node, got: %q", h.Reason)
	}
}

func TestHint_NilFetcher_OfflineMode(t *testing.T) {
	// Even with a file path and symbol, nil fetcher = offline, no escalation.
	h := graphify.Hint(
		"migrate src/app/Services/Payment/CreateSubscriptionStructureService.php to v5",
		graphify.DefaultCriteria(),
		nil, // offline
	)
	if h.ShouldEscalate {
		t.Errorf("nil fetcher should never escalate, got: %q", h.Reason)
	}
}

func TestHint_Reason_NonEmpty_WhenEscalated(t *testing.T) {
	fetcher := stub(map[string]graphify.NodeInfo{
		"CreateSaleService": {
			Label:     "CreateSaleService",
			Community: "Client & Subscription State",
			Edges:     33,
			Found:     true,
		},
	})

	h := graphify.Hint("debug CreateSaleService", graphify.DefaultCriteria(), fetcher)
	if !h.ShouldEscalate {
		t.Skip("no escalation; skipping reason check")
	}
	if h.Reason == "" {
		t.Error("Reason must be non-empty when ShouldEscalate is true")
	}
}

func TestExtractCandidates_OnlyFilePaths(t *testing.T) {
	// Indirectly tested via Hint — no file path → no fetcher call.
	h := graphify.Hint("rename the userId variable", graphify.DefaultCriteria(), stub(nil))
	if h.ShouldEscalate {
		t.Error("pure text prompt should not trigger graph lookup or escalation")
	}
}

func TestCmdFetcher_EscalatesOnHighEdges(t *testing.T) {
	script := filepath.Join(t.TempDir(), "fetch.sh")
	body := "#!/bin/sh\ncat >/dev/null\nprintf '%s\\n' '{\"label\":\"AuthService\",\"community\":\"erp controllers\",\"edges\":30,\"found\":true}'\n"
	if err := os.WriteFile(script, []byte(body), 0o700); err != nil {
		t.Fatal(err)
	}
	h := graphify.Hint("debug CreateSaleService", graphify.DefaultCriteria(), graphify.CmdFetcher{Cmd: script})
	if !h.ShouldEscalate {
		t.Fatalf("expected escalation from command fetcher, got %+v", h)
	}
}

func TestCmdFetcher_FailOpen(t *testing.T) {
	h := graphify.Hint("debug CreateSaleService", graphify.DefaultCriteria(), graphify.CmdFetcher{Cmd: "false"})
	if h.ShouldEscalate {
		t.Fatalf("failed command must not escalate, got %+v", h)
	}
}

// A hung DOWNSHIFT_GRAPHIFY_CMD must not stall routing past the hook
// deadline, even with several candidate labels in the prompt.
func TestCmdFetcher_HungCommandBoundedByHookDeadline(t *testing.T) {
	if _, err := exec.LookPath("sleep"); err != nil {
		t.Skip("sleep not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	defer hookctx.Set(ctx)()
	prompt := "fix src/a.go src/b.go src/c.go PaymentService UserController"
	start := time.Now()
	hint := graphify.Hint(prompt, graphify.DefaultCriteria(), graphify.CmdFetcher{Cmd: "sleep 10"})
	if hint.ShouldEscalate {
		t.Fatal("a failed fetch must never escalate")
	}
	if d := time.Since(start); d > time.Second {
		t.Fatalf("graphify lookups took %s past a 300ms deadline", d)
	}
}
