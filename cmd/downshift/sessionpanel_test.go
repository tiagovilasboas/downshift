// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/execwrap"
	"github.com/tiagovilasboas/downshift/internal/paths"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// The compactor block totals the whole exec log: runs of this session, of
// another session and with no session at all. The raw session id never
// reaches the panel, and an empty log carries no block.
func TestPanel_CompactorIsTheTotalOfAllRuns(t *testing.T) {
	t.Setenv(paths.EnvStateDir, t.TempDir())
	t.Setenv(execwrap.DepthEnv, "") // clear in case we're running inside `downshift compactor`
	for _, n := range core.AllHarnessCaps() {
		if n.Caps.SessionEnv != "" {
			t.Setenv(n.Caps.SessionEnv, "")
		}
	}
	empty, _ := json.Marshal(panelWithCompactor(telemetry.PanelView{Tokens: telemetry.TokenSaving{WorkSaved: 40}}))
	if strings.Contains(string(empty), "compactor") {
		t.Fatalf("an empty exec log must carry no compactor block: %s", empty)
	}
	if !strings.Contains(string(empty), `"tokens_saved_estimate":40`) {
		t.Fatalf("routing-only total missing: %s", empty)
	}

	bin := t.TempDir()
	var suite strings.Builder
	for i := 0; i < 8; i++ {
		fmt.Fprintf(&suite, "ok  \tsecret/pkg/p%d\t0.1s\n", i)
	}
	if err := os.WriteFile(filepath.Join(bin, "suite.txt"), []byte(suite.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\ncat \"$(dirname \"$0\")/suite.txt\"\n"
	if err := os.WriteFile(filepath.Join(bin, "go"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	mine, other := telemetry.HashSessionID("s-mine"), telemetry.HashSessionID("s-other")
	for _, args := range [][]string{
		{"--session=" + mine, "--"},
		{"--session=" + other, "--"},
		{"--"},                   // a direct call with no session
		{"--session=../x", "--"}, // a malformed session is ignored, the run still counts
	} {
		var out, errb bytes.Buffer
		if rc := runExec(append(args, "go", "test", "./secret/..."), strings.NewReader(""), &out, &errb); rc != 0 {
			t.Fatalf("rc = %d, stderr %q", rc, errb.String())
		}
	}

	raw, err := json.Marshal(panelWithCompactor(telemetry.PanelView{Session: mine}))
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Compactor           *compactorPanel `json:"compactor"`
		TokensSavedEstimate int64           `json:"tokens_saved_estimate"`
	}
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got.Compactor == nil || got.Compactor.Scope != "all" || got.Compactor.Commands != 4 || got.Compactor.Compacted != 4 || got.Compactor.SavedBytes <= 0 {
		t.Fatalf("compactor block = %+v, want 4 compacted runs across every session, scope all", got.Compactor)
	}
	if want := got.Compactor.EstimatedTokensSaved; got.TokensSavedEstimate != want || want <= 0 {
		t.Fatalf("tokens_saved_estimate = %d, want the compactor estimate %d (no routing tokens)", got.TokensSavedEstimate, want)
	}
	view := telemetry.PanelView{Session: mine, Tokens: telemetry.TokenSaving{WorkSaved: 300, CacheSaved: 200}}
	both := panelWithCompactor(view)
	if both.TokensSavedEstimate != 500+got.Compactor.EstimatedTokensSaved {
		t.Fatalf("both = %d, want routing 500 + compactor %d", both.TokensSavedEstimate, got.Compactor.EstimatedTokensSaved)
	}
	if strings.Contains(string(raw), "s-mine") || strings.Contains(string(raw), "s-other") {
		t.Fatalf("a raw session id leaked into the panel: %s", raw)
	}
}
