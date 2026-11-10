// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/adapters/claudecode"
	"github.com/tiagovilasboas/downshift-ai/internal/core"
	"github.com/tiagovilasboas/downshift-ai/internal/hookport"
	"github.com/tiagovilasboas/downshift-ai/internal/postrun"
)

// A result over the 1 MiB hook cap is still compacted where the harness can
// replace it: the post-run hook reads up to postRunPayloadCap. Every other
// harness, and the post-run hook when it is off, keeps the 1 MiB cap and
// answers {} without reading further.
func TestPostRun_PayloadOverOneMiB(t *testing.T) {
	postRunEnv(t, true)
	stdout := verboseGoTest(40000) // about 2.1 MB
	payload := bashPost(t, "go test -v ./... 2>&1 | cat", stdout, "")
	if int64(len(payload)) <= hookPayloadCap || int64(len(payload)) >= postRunPayloadCap {
		t.Fatalf("payload %d bytes, want between the two caps", len(payload))
	}
	_, got := honorAnswer(t, portFor("claude-code"), payload)
	out, _ := got["stdout"].(string)
	if got == nil || len(out) >= len(stdout)/10 || !strings.Contains(out, "downshift retrieve ") {
		t.Fatalf("claude-code: %d bytes from %d, want a cut naming the retrieve id", len(out), len(stdout))
	}

	noCap := hookport.Port{
		ID:                "cursor",
		Honor:             func([]byte, string, core.Resolver) (any, string) { return nil, "" },
		ParseToolOutput:   claudecode.ParseToolOutput,
		ReplaceToolOutput: claudecode.ReplaceToolOutput,
	}
	if answer, got := honorAnswer(t, noCap, payload); got != nil || strings.TrimSpace(answer) != "{}" {
		t.Errorf("harness without the capability read past 1 MiB: answer %q", answer[:min(len(answer), 80)])
	}
	t.Setenv(postrun.Env, "0")
	if honorPayloadLimit("claude-code") != hookPayloadCap {
		t.Errorf("%s=0 must keep the 1 MiB cap", postrun.Env)
	}
}
